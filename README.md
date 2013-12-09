# codremaster

codremaster is a plug and play master server replacement for vCOD, COD2 and COD4. It comes with a web interface for easy server browsing and an API endpoint to query servers for their status.

## Differences between vCOD, COD2 and COD4

For vCOD codremaster needs to listen on ports 20510 and 20500 (master and auth server), for COD2 on 20710 and 20700 and finally for COD4 on 20810 and 20800.

## Usage
### Clients

Players have to update their hosts file to use a codremaster server. Depending on the game the following change has to be made:

`127.0.0.1 cod(2|4)?master.activision.com`

Use codmaster.activision.com for vCOD, cod2master.activision.com for COD2 and cod4master.activision.com for COD4.

Players can make this change now, while the official servers are still running, or wait until they are offline for good. In the meantime codremaster will relay to the official server*.

\*feature not yet implemented

### Game Servers

If you run a game server and want to be listed on a codremaster server add the following line to the server config:

`set sv_master9 "cod2master.activision.com`

Where the number after sv\_master is any number not yet used for a master in your config.

## Licence
codremaster is released under the MIT license. See LICENSE for details.
