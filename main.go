package main

import "errors"
import "flag"
import "fmt"
import "io/ioutil"
import "log"
import "net"
import "net/http"
import "os"
import "time"

import "github.com/prasmussen/gandi-api/client"
import "github.com/prasmussen/gandi-api/domain/zone"
import "github.com/prasmussen/gandi-api/domain/zone/record"
import "github.com/prasmussen/gandi-api/domain/zone/version"

var (
	apiKey       string
	testPlatform bool
	zoneId       int64
	recordName   string
	refresh      time.Duration
)

// Define and parse flags
func init() {
	flag.StringVar(&apiKey, "apikey", "", "Mandatory. API key to access server platform")
	flag.BoolVar(&testPlatform, "test", false, "Perform queries against test platform (OT&E) instead of production platform")
	flag.Int64Var(&zoneId, "zone", 0, "Mandatory. Zone id")
	flag.StringVar(&recordName, "record", "", "Mandatory. Record to update")
	flag.DurationVar(&refresh, "refresh", 5*time.Minute, "Delay between checks for public IP address updates")
}

// Returns the public IP address
func getPublicIP4() (string, error) {
	res, err := http.Get("http://api.externalip.net/ip/")
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	data, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	ip := net.ParseIP(string(data))
	if ip == nil || ip.To4() == nil {
		return "", errors.New("no ipv4 valid address")
	}
	return ip.String(), nil
}

// Delete a version of a DNS zone
func deleteVersion(client *client.Client, zoneId, versionId int64) {
	if _, err := version.New(client).Delete(zoneId, versionId); err != nil {
		log.Println("Warning: failed to delete version", versionId, ":", err)
	}
}

// Get the RecordInfo of a version of a DNS zone
func getRecord(client *client.Client, zoneId, versionId int64, recordName string) (*record.RecordInfo, error) {
	records, err := record.New(client).List(zoneId, versionId)
	if err != nil {
		return nil, err
	}
	for _, r := range records {
		if r.Name == recordName {
			return r, nil
		}
	}
	return nil, errors.New("record not found")
}

// Update the record using the provided RecordInfo.
// A new version of the zone will be created and activated.
func updateRecord(client *client.Client, zoneId, versionId int64, r *record.RecordInfo) (int64, error) {
	updateFailed := errors.New("update failed")
	// Copy current version as a new one
	newVersion, err := version.New(client).New(zoneId, versionId)
	if err != nil {
		log.Println("Error: failed to create new version:", err)
		return 0, updateFailed
	}
	// Get the previous record in the new version
	oldRecord, err := getRecord(client, zoneId, newVersion, r.Name)
	if err != nil {
		log.Print("Error: failed to get old record:", err)
		// Rollback new version creation
		deleteVersion(client, zoneId, newVersion)
		return 0, updateFailed
	}
	// Delete previous record in this new version
	if ok, err := record.New(client).Delete(zoneId, newVersion, oldRecord.Id); !ok {
		log.Print("Error: failed to delete previous record:", err)
		// Rollback new version creation
		deleteVersion(client, zoneId, newVersion)
		return 0, updateFailed
	}
	// Add updated record
	recordAdd := record.RecordAdd{zoneId, newVersion, r.Name, r.Type, r.Value, r.Ttl}
	if _, err := record.New(client).Add(recordAdd); err != nil {
		log.Print("Error: failed to add updated record:", err)
		// Rollback new version creation
		deleteVersion(client, zoneId, newVersion)
		return 0, updateFailed
	}
	// Activate new version
	if _, err := version.New(client).Set(zoneId, newVersion); err != nil {
		log.Println("Error: failed to activate updated version:", err)
		// Rollback new version creation
		deleteVersion(client, zoneId, newVersion)
		return 0, updateFailed
	}
	// Delete old version
	deleteVersion(client, zoneId, versionId)
	return newVersion, nil
}

func main() {
	flag.Parse()
	if apiKey == "" || recordName == "" || zoneId == 0 {
		fmt.Println("Missing one or more command line options.")
		flag.PrintDefaults()
		os.Exit(2)
	}

	platform := client.Production
	if testPlatform {
		platform = client.Testing
	}

	// Get the active version of the zone
	client := client.New(apiKey, platform)
	zoneInfo, err := zone.New(client).Info(zoneId)
	if err != nil {
		log.Println("Error: could not get current version:", err)
		return
	}
	activeVersion := zoneInfo.Version

	// Get registered ip address
	recordInfo, err := getRecord(client, zoneId, activeVersion, recordName)
	if err != nil {
		log.Println("Error: could not get current record:", err)
		return
	}
	registeredIp := recordInfo.Value
	log.Println("Info: current registered IP:", registeredIp)

	for {
		// Get the current public address
		currentIp, err := getPublicIP4()
		if err != nil {
			log.Println("Error: failed to get pulic IP:", err)
		} else if currentIp != registeredIp {
			// Update Gandi record when IP changes
			recordInfo.Value = currentIp
			if newVersion, err := updateRecord(client, zoneId, activeVersion, recordInfo); err == nil {
				activeVersion = newVersion
				registeredIp = currentIp
				log.Print("Info: updated Gandi records with IP:", currentIp)
			}
		}
		time.Sleep(refresh)
	}
}
