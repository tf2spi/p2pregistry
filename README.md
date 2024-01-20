# p2pregistry

Simple HTTP(S) Web Server serving as a service registry for
peer-to-peer (p2p) applications like video games or file shares

It's very easy to setup and doesn't even require a database.
The peer information is stored entirely in memory.

## Use Case

When making something like a video game, it's common to have
clients connect to each other via NAT hole punching. So long
as the peers know what IP addresses the other peers expose, they
can connect to each other.

How do they know where to get the IP addresses? That's often
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

* timestamp - Only return peers that registered during or later than this timestamp (default: 0)
* prefer - '4' or '6', which chooses whether ipv4 or ipv6 addresses respectively are returned first (default: 4)
* subnet4 - CIDR notation for the subnet to use when filtering IPv4 addresses (default: 0.0.0.0/0)
* subnet6 - CIDR notation for the subnet to use when filtering IPv6 addresses (default: ::/0)

This queries the current list of peers, filtering them using the parameters.

It returns a json object with the current time to store for later use with
the timestamp parameter and the current list of peers. Here's an example below.

{"now":42,"peers":["10.123.42.24","fe80::fe24"]}

--------------
POST /register
--------------

This request takes no parameters and registers the requester as a peer.

Peers have the following fields

* ip: IP address of the registered peer
* expires: The number of seconds left before the peer's registration expires
* timestamp: The timestamp when the peer last registered after expiration

If the peer is already registered with the service,
this request will refresh the expiration time and keep
the old timestamp of the peer.

If the peer is not registered, it will use the current time,
the current expire period, and the IP address of the requester
to register the new peer.

Note that this means you cannot use any sort of proxy. This is done
to avoid spoofed IP addresses hogging up the memory of the server.

This request returns the fields of the peer as a json object.
An example is shown below.

{"ip":"10.123.42.24","timestamp":42,"expires":60}

```

