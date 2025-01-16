# infoblox for [`libdns`](https://github.com/libdns/libdns)

This package implements the [libdns interfaces](https://github.com/libdns/libdns)
for [infoblox](https://www.infoblox.com), allowing you to manage DNS records.

## Authenticating
The following parameters are used to authenticate with the Infoblox API:
* `Host` - The hostname of the Infoblox server, e.g. `infoblox.example.com`
* `Version` - The version of the Infoblox API, e.g. `2.9.7`
* `Username` - The username to authenticate with
* `Password` - The password to authenticate with

## Supported Record Types
I'm really only using this for ACME DNS-01 challenges, so only `TXT` and `CNAME` records are supported. Feel free to open a PR to add more record types.