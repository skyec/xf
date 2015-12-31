# Experiment 1: Parallel Requests

Attempt downloading a large (10MB) file using both TCP (HTTP) and UDP (custom)
over an unreliable connectyion. Make multiple, concurrent rerquests to the server
for the various file parts then assemble the final file in the end.


## TCP 1

- Use a static IP address
- Connect to a server once
- Issue a GET request for the full file
- Report # of bytes as it downloads

## TCP

- Use a static IP address
- Connect to a server x4
- For each connection:
  - Issue an HTTP GET
  - Report # bytes as it downloads for each chunk
  - Report # of completed chunks


## UDP

- Use a static IP address
- Send a request for each chunk
- Server responds with that chunk
- Client handles timeouts and packet loss
  - Track # of timeouts/retrans requests
  - Track # of bytes as it reads each response 
    (is it ever less than the expected chunk sizse?)
  - Track # of completed chunks
 
## Extra

Add a few extra features to the server:

- HTTP endpoint to generate a test file and return it's sha1 checksum
- HTTP endpoint to return info about the test file:
  - sha1sum
  - file size
  - number of chunks

## HTTP Endpoints

Request:
```
POST /newtest
```
Response:
```
{"sha1sum":"asdfasfasdf"}
```

Request:
```
GET /file
```

Response:
```
{
        "sha1sum":"asdfasdasdf",
        "sizeB":123123123,
        "chunks":123,
}
```

Request:
```
GET /file/asdfasdfasdf
```

Response:
The whole file

Request:
```
GET /file/asdfasdfasdf/1
```

Response:
The whole chunk for file asdfasdfasdf


