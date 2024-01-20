# p2pregistry

Simple HTTP(S) Web Server serving as a service registry for
peer-to-peer (p2p) applications like video games or file shares

## Use Case

When making something like a video game, it's common to have
clients connect to each other via NAT hole punching. So long
as the peers know what IP addresses and ports the other peers expose,
they can connect to each other.

How do clients know where to get the IP addresses and ports? That's often
application-specific. However, this web server aims to provide
an easy way for peers to broadcast themselves without the application
having to do anything except send some basic heartbeats over HTTP.

## Usage

TODO:
Parse command line arguments so that users can bind to different
addresses and ports as well as configure expiration periods and timeouts.

Also find out how to do HTTPS with ``net/http``.

Until then, just run it like this
```sh
go run .
```

## Protocol

```
---------------
GET /query
---------------

This request takes the following parameters as a query string:

* port - Only return peers with this port number. Ignored if port is 0. (default: 0)
* timestamp - Only return peers that registered during or later than this timestamp (default: 0)
* prefer - '4' or '6', which chooses whether ipv4 or ipv6 addresses respectively are returned first (default: 4)
* subnet4 - CIDR notation for the subnet to use when filtering IPv4 addresses (default: 0.0.0.0/0)
* subnet6 - CIDR notation for the subnet to use when filtering IPv6 addresses (default: ::/0)

This queries the current list of peers, filtering them using the parameters.

It returns a json object with the current time to store for later use with
the timestamp parameter and the current list of peers. Here's an example below.

{"now":42,"peers":["10.123.42.24:80","[fe80::fe24]:443"]}

--------------
POST /register
--------------

This request takes one optional parameter listed below as a urlencoded form

* port - Port the requester is listening on for connections (Default: 443)

Peers have the following fields

* peer: IP address and port of the registered peer
* expires: The number of seconds left before the peer's registration expires
* timestamp: The timestamp when the peer last registered after expiration

If the peer is already registered with the service
and the peer has not changed ports, this request will
refresh the expiration time and keep the old timestamp of the peer.

If the peer is not registered or they changed ports, it will use
the current time, the current expire period, the IP address and
new port of the requester to register the new peer.

Note that this means you cannot use any sort of proxy and you can
only register one port with this service. This is done to avoid spoofed
IP addresses and ports hogging up the memory of the server.

This request returns the fields of the peer as a json object.
An example is shown below.

{"peer":"10.123.42.24:8080","timestamp":42,"expires":60}
```

## Current Limitations

* Only one port can be registered per peer. The workaround for this is to
  use that registered port as a reverse proxy for other services or for
  virtual networking.

* There is no persistence mechanism to store registered peers. Everything
  is stored in memory. If the server is killed, all peers are lost and they
  must re-register again under a new timestamp. Storage is technically not
  needed because registering is ephemeral and has to be renewed constantly.
  This only affects caching mechanisms when querying. (TODO: Something
  basic like a CSV would be sufficient).

* The schema is very basic so peers have to make separate connections to
  others to do more complicated filtering logic. This incurs a loss in performance
  compared to having a more complicated schema in the service registry so only
  one connection needs to be made for filtering instead of many connections.

