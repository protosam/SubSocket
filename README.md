# SubSocket (Prototype)
A subscription based messaging over websockets.

## Testing it out
From the commandline, in the SubSocket repo, run:
```
  go run subsocket.go
```
Open up your browser to `localhost:8080/static/` and there's testing instructions on that page. If you follow that from top to bottom, it should give a hands on illustration of what we're trying to achieve.

## At a Glance
This is a proof-of-concept that shows how the messaging system would function. The design goal is to separate the messaging logic between the server and the application. Doing it in with subscription based model should open up some ability to offload non-critical work onto clients.

Also a very small REST API exists to make admin clients capable of orchestrating the flow of messages.

A Diagram, because visual aids...:

```
+-------------------------+           +------------------------------+
| Client1 with subs:      |           |       SubSocket Server       |
| game_logic_channel      +---------->+                              |
|                         |           |                              |
+-------------------------+           |                              |
+-------------------------+   +------>+                              |
| Client2 with subs:      |   |       |                              |
| game_logic_channel      +---+  +--->+                              |
|                         |      |    |                              |
+-------------------------+      |    |                              |
+-------------------------+      |    +------------------------------+
| Admin_Client with subs: +------+
| game_logic_channel      |
|                         |
+-------------------------+
```

## API
The API is bare minimum. Things that it needs are like the ability to `kick` and `ban`. If expanded, a user auth checking system can be implemented with API configuration. 

API Documentation can be found here: https://documenter.getpostman.com/view/6063841/T1LJkoh6

## Life-Cycle
This is still up for debate, even has room for improvements, but it works. I think this can be expanded to be a massive swiss-army knife. Hopefully this helps clarify how I'm imagining this working. I'm using `admin-client.html` as my test client, if you open it up it has instructions to run from the developer console in the browser.

1. SubSocket is already running.
- Admin_Client connects
- Admin_Client creates "game-system" as an open channel via REST API
- Admin_Client creates "game-global" as an open channel via REST API
- Admin_Client subscribes to the "game-system" to recieve messages from users. Users can send to this channel without subscribing. It's effectively private.

2. Clients will connect and subscribes to "game-system"
- SubSocket announces the new connection to all is-admins providing UUID
- At this point the client is responsible for publishing commands to the "game-system" channel for the Admin_Client to process
- The client will receive any messages sent to the channels it is subscribed to.
- The client will receive any messages sent by Admin_Client via UUID. 

3. Admin_Client recieves connection announcement from SubSocket to decide how to handle. Maybe you want to auto subscribe this user to some channels with `/api.v1/clients/subscribe?channel=game-global&id=CLIENT_UUID` in the rest API.
- Admin_Client can now directly message the connected client via it's UUID.
- Admin_Client can subscribe/unsubscribe Client1 in channels.

4. Eventually the client disconnects.
- SubSocket will remove the client from all the channels it was subscribed to.
- SubSocket will notify Admin_Client about the disconnection with associated UUID.


Now what this has been leading up to is the ability to have a server as a service, where server logic is just another client. The idea of a client being a host is not new and neither is the Pub/Sub concept. However I think this model is flexible and a novel way to do server architecture.

This further advances bringing user/host hybrids to offload some processing to the browser for more intense workloads. The only distributed browser workload I've seen so far was a crypto-miner, it's not the same though. 

## Persistent Storage
Admin clients are repsonsible for maintaining server-side states and storage. SubSocket doesn't provide this functionality to keep things simple. 

## Where to run Admin Clients
You can run admin clients where ever you want, as long as they can maintain a persistent connection to where you're hosting the SubSocket server. Admin clients can be separate daemons that starts up after SubSocket on the same server, using systemd unit files or supervisord or whatever. 

Something I've already eluded to was distributing some of the workload client side.

## Securing SubSocket
SubSocket is insecure without taking the neccessary infrastructure steps. Everything is served over HTTP (no SSL), however you can proxy to the API and WebSocket with Apache and Nginx to serve this up over SSL. You should also block direct access with your system firewall. If you are using docker, it should be secure with just Apache/Nginx exposed.

## What's Next?
I have been really liking how this prototype is looking, so I published it to see if anyone is interested in this existing. If the reception is positive and people find it useful, I will persue rewriting it to be reliable. 

Some use cases I see this being useful for are offloading some non-critical work to the browsers on sites that have purely server/client reliationships. That being online coop gameplay, 

Anyone wanting to contribute is welcome to reach out in the `Issues` section. Help is welcomed, I could use input on the code design and assistance in writing good Go tests. I pretty much slammed keys overnight to get this prototype into this state, it's very small to show an idea, but it's not optimized at all. I also don't even know if I used locks in all the right places, I just didn't want to switch the maps to sync.Map. Lastly it's not configurable, the `AdminToken` is defined in the code and not pulled from a config file. 
