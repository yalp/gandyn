## Overview 

A dynamic IP updater for Gandi.

It simply polls a public ip API and updates the Gandi DNS records
using the [Gandi RPC APIs](http://doc.rpc.gandi.net/).

## Install

With the go tools:

    $ go get github.com/yalp/gandyn
    $ go install github.com/yalp/gandyn

## Usage

Some infos are required to use gandyn:
- the API key from the [admin interface](https://www.gandi.net/admin/api_key)
- the zone ID of the domain to update
- the name of the record to update (like "www" or "blog")

The record must already exist on the active version of the zone before gandyn can update it.

Just launch it at startup as a service on startup, or using cron :

    @reboot /path/to/gandyn -apikey "XXX" -zone 6666 -record "www"

Once started, gandyn will keep running indefinetly until stopped.

## Options

* `-apikey`, `-zone`, `-record` are mandatory (see above)
* `-refresh` defines the delay for polling the pulic IP address
* `-test` to use the test OT&E platform instead of production platform
