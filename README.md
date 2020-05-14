# COVID19 - Contact Tracing
[![Build Status](https://drone.bryk.io/api/badges/bryk-io/ct19/status.svg)](https://drone.bryk.io/bryk-io/ct19)
[![Version](https://img.shields.io/github/tag/bryk-io/ct19.svg)](https://github.com/bryk-io/ct19/releases)
[![Software License](https://img.shields.io/badge/license-BSD3-red.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/bryk-io/ct19?style=flat)](https://goreportcard.com/report/github.com/bryk-io/ct19)

We provide an open-source, privacy respecting and security oriented
"Contact Tracing" platform, that could be use as an auxiliary tool
in the health contingency caused by the COVID-19 pandemic.

According to the World Health Organization:

> People in close contact with someone who is infected with a virus, such
> as the Ebola virus, are at higher risk of becoming infected themselves,
> and of potentially further infecting others.
>
> Closely watching these contacts after exposure to an infected person will
> help the contacts to get care and treatment, and will prevent further
> transmission of the virus.
>
> This monitoring process is called contact tracing, which can be broken
> down into 3 basic steps:
>
> __Contact identification:__ Once someone is confirmed as infected with
> a virus, contacts are identified by asking about the person’s activities
> and the activities and roles of the people around them since the onset
> of illness. Contacts can be anyone who has been in contact with an
> infected person: family members, work colleagues, friends, or health care
> providers.
>
> __Contact listing:__ All persons considered to have contact with the
> infected person should be listed as contacts. Efforts should be made
> to identify every listed contact and to inform them of their contact
> status, what it means, the actions that will follow, and the importance
> of receiving early care if they develop symptoms. Contacts should also
> be provided with information about prevention of the disease. In some
> cases, quarantine or isolation is required for high risk contacts,
> either at home, or in hospital.
>
> __Contact follow-up:__ Regular follow-up should be conducted with all
> contacts to monitor for symptoms and test for signs of infection.

To be effective, a contact tracing campaign should incorporate comprehensive,
scalable and robust technological tools; as well as robust public policy
considerations. The objective of this platform is to facilitate the technical
aspects of it.

## Architecture

To simplify operations and improve scalability and flexibility, the platform
is deployed as several independent components.

- __API Server:__ Receive and handle all user requests, manage access
  credentials and authentication/authorization requirements.
- __Worker(s):__ Asynchronously handle potentially time-consuming tasks.
- __Storage:__ Provides an external persistent data store and handle specific
  queries and general data retrieval. The platform can be deployed using
  different storage mechanisms.
- __Broker:__ Distribute messages (using the "Advanced Message Queue
  Protocol") to other components in the platform and allow for horizontal
  scaling. A specific deployment can also incorporate any number of
  additional operators to receive and process events and notifications
  generated on the platform. This allows for greater flexibility and
  facilitate interoperability with external applications.
- __Client(s):__ Usually a mobile application. Track user location and
  dispatch events. Receive notifications from the API server. There can
  be several client applications.

## Installation

The platform is distributed as single statically compiled binary and
uses a single YAML (or JSON) configuration file. By default, the configuration
file will be loaded from `/etc/ct19/config.yml` if available.

Example configuration file:

```yaml
storage: mongodb://localhost:27017
broker: amqp://localhost:5672/ct19
server:
  name: sample-ct19.iadb.org
  home: /etc/ct19
  port: 9090
resolver:
  - method: iadb
    endpoint: https://did.iadb.org/v1/retrieve/{{.Method}}/{{.Subject}}
    protocol: http
  - method: ccp
    endpoint: https://did.baidu.com/v1/did/resolve/{{.DID}}
    protocol: http
  - method: stack
    endpoint: https://core.blockstack.org/v1/dids/{{.DID}}
    protocol: http
```

To start an API server instance simply run the following CLI command. The
example assumes the configuration file is on `/home/user/ct19-conf.yml`
instead of the default location.

```bash
ct19 server --config /home/user/ct19-conf.yml
```

To start a worker instance use the following.

```bash
ct19 worker --config /home/user/ct19-conf.yml
```

## Security
Platform security is defined as privacy, authentication and authorization
considerations. In terms of privacy, no personally-identifiable information
is ever used or collected. For tasks requiring authentication mechanisms,
anonymous bearer credentials are used in the form of JWT - ES384 tokens.
Finally, authorization requirements are managed using an RBAC (i.e., Role-Based
Access Control) model.

There are 3 roles for users on the platform:
- __Administrator:__ To manage and handle all assets on the platform.
- __Agent:__ Agents represent official representatives of the health system
  and can generate notification requests. Based on the requirements for a
  particular setup (by country, state, city, etc), an agent can represent
  an individual hospital, an individual healthcare professional, etc.
- __User:__ Use a client application to send location records and receive
  notifications.

Sample access credential (line breaks added for readability).

```
eyJ0eXAiOiJKV1QiLCJhbGciOiJFUzM4NCIsImtpZCI6IjkzOjkxOjliOmZmOjJiOmJmOjc4OjE6ZW
Y6MWU6NWM6OGY6ZjE6MmU6NmU6OWUifQ.eyJhdWQiOlsic2FtcGxlLWN0MTkuYnJ5ay5pbyJdLCJka
WQiOiJkaWQ6YnJ5azo3ODg5Yzk2NS00NjQ0LTQ0ZmYtYjc2MC1mMzk2ZjFkMTE0NDQiLCJleHAiOjE
1ODkxNTE3ODIsImlhdCI6MTU4ODU0Njk4MiwiaXNzIjoic2FtcGxlLWN0MTkuYnJ5ay5pbyIsImp0a
SI6ImI2NjhhNmFjLTg5ZDItNGIzYy1iZDA5LTE5NDhiNzM4MjIyMiIsIm5iZiI6MTU4ODU0Njk4Miw
icm9sZSI6ImFkbWluIiwic3ViIjoiZGlkOmJyeWs6Nzg4OWM5NjUtNDY0NC00NGZmLWI3NjAtZjM5N
mYxZDExNDQ0In0.xx2j7_Yz9IXwRHHZly5UWBIOwgnTvllipC3l0tDhoQwWQTk76z2ZsmiGha34QyW
UucMGmwKYUuQVjfd4TaV7Gnogs3Jr6plo4jtQ_oaAVFMydapU_yuPxM61YNmdXgrh
```

The header section of the credential specify its type.

```json
{
  "typ": "JWT",
  "alg": "ES384",
  "kid": "93:91:9b:ff:2b:bf:78:1:ef:1e:5c:8f:f1:2e:6e:9e"
}
```

While the payload section include only the standard JWT private attributes,
plus the user's DID and role.

```json
{
  "aud": [
    "sample-ct19.iadb.org"
  ],
  "did": "did:iadb:7889c965-4644-44ff-b760-f396f1d11444",
  "exp": 1589151782,
  "iat": 1588546982,
  "iss": "sample-ct19.iadb.org",
  "jti": "b668a6ac-89d2-4b3c-bd09-1948b7382222",
  "nbf": 1588546982,
  "role": "agent",
  "sub": "did:iadb:7889c965-4644-44ff-b760-f396f1d11444"
}
```

All identification and authentication operations are performed using
Decentralized Identifiers (DID). These identifiers present the following
considerations.

- Anyone must have access to freely register, publish and update as many
  identifiers as considered necessary.
- There should be no centralized authority required for the generation and
  assignment of identifiers.
- The end user must have true ownership of the assigned identifiers, i.e.
  no one but the user should be able to remove, revoke and/or reassign the
  user's identifiers.

The user's DID is used to digitally sign all provided information, like
location records, in a secure, tamper-proof and verifiable way. These
cryptographic proofs of provenance and integrity are usually represented
in JSON-LD format.

Sample JSON-LD proof.

```json
{
  "@context": [
    "https://w3id.org/security/v1"
  ],
  "type": "Ed25519Signature2018",
  "creator": "did:iadb:7889c965-4644-44ff-b760-f396f1d11444#master",
  "created": "2020-05-01T15:47:55Z",
  "domain": "sample-ct19.iadb.org",
  "nonce": "462cf6c3817e5e28919a3abe11e30bdd",
  "signatureValue": "cStV/85EQpGEWX8/+mQrYdnhplGiILYayuZMZ8Wd4vkDDcnmbzBeK6/NnDVVMJm0crqxPySizTHnVSazQclvAQ=="
}
```

For more information on the DID specifications refer to the
[W3C Community Working Group](https://w3c.github.io/did-core/).

## User Tracking

A user continuously monitors and reports his/her location utilizing a client
application. The general process is the following.

- The application gets the user location and generates a record containing the
  following details.
    - Timestamp
    - GPS location
    - Device identifier
    - Digital signature (encoded as a JSON-LD cryptographic proof document)
- The location record is send to the API server. The processing of a message
  is the following.
    - The message is placed on a processing queue by the API server.
    - The message is retrieved by a worker.
    - The format is validated.
    - The device identifier is verified, i.e., the DID is resolved and retrieved.
    - The digital signature is verified.
    - The location record is placed on the queue for storage.
    - When stored, the location record is indexed by its timestamp and location.
- The application will keep generating location records on the background every
  60 seconds, or when the user's location change. Records that need to be reported
  will be locally stored if the device doesn't have an internet connection and
  send in batch once a connection is available. Each batch can include a maximum
  of 100 individual records.

## User Notification
When a user is tested and identified as a positive COVID-19 case by an official
health care professional (i.e. the agent) a notification should be generated.

- Using her application the agent scans the QR identifier on the user's phone.
  The application will generate a notification request with the following details.
    - Timestamp
    - GPS location
    - Patient's device identifier
    - Agent's device identifier
    - Digital signature
- The notification request is send to the API server for validation.
    - Validate request format
    - Validate agent credentials
    - Validate digital signature in the request is valid
    - Validate a notification has not been already dispatched for the same patient
    - Place the notification in the queue for processing
- Valid notification are processed in the order they are placed in the queue
  (i.e., FIFO).
- The API server request a list of all identifiers that have been a meet record
  with the patient in the 3 weeks.
- The API server request a list of all identifiers that have been in proximity
  with the patient based on its ping records.
- The server dispatch notifications to all the users at risk using the available
  delivery mechanisms, for example: Push notifications, In-app notifications, etc.

## API

The main way to communicate with the platform is through the public API.
The server provides an HTTP2 and HTTPS interface to expose the available
functionality. When using HTTPS all data provided to, and returned by,
the server is encoded in JSON format.

For methods requiring authentication, the credentials must be provided
as a bearer token using the `Auhentication` HTTP header.

```
Authorization: Bearer "...JWT token goes here..."
```

For example, to submit a new location record to be processed. Send a JSON
document like the following as an authenticated POST operation to the endpoint
`/v1/api/record` on the API server.

```json
{
  "records": [
    {
      "did": "did:iadb:7889c965-4644-44ff-b760-f396f1d11444",
      "lat": -77.08672,
      "lng": 38.862846,
      "alt": 0,
      "timestamp": "1588619270",
      "hash": "57365a03a9550e46aba13de8840f4674aaf198f506d0251b48c55c994efc0602",
      "proof": "ewogICJAY29udGV4...|shortened for readability|...RzhWY0xCQT09Igp9"
    }
  ]
}
```

The complete (and latest) version of the OpenAPI/Swagger specification is
[available here.](https://github.com/bryk-io/ct19/blob/master/proto/v1/tracking_server_api.swagger.json)
The available API methods are the following.

### /v1/api/ping

Basic reachability test.

```json
{
    "/v1/api/ping": {
      "get": {
        "responses": {
          "200": {
            "description": "A successful response.",
            "schema": {
              "$ref": "#/definitions/v1PingResponse"
            }
          }
        }
      }
    }
}
```

### /v1/api/record

Process location record events. A maximum value of 100 record per-request is enforced.

```json
{
    "/v1/api/record": {
      "post": {
        "responses": {
          "200": {
            "description": "A successful response.",
            "schema": {
              "$ref": "#/definitions/v1RecordResponse"
            }
          }
        },
        "parameters": [
          {
            "name": "body",
            "in": "body",
            "required": true,
            "schema": {
              "$ref": "#/definitions/v1RecordRequest"
            }
          }
        ]
      }
    }
}
```

### /v1/api/activation_code

Generate a new activation code.

```json
{
    "/v1/api/activation_code": {
      "post": {
        "responses": {
          "200": {
            "description": "A successful response.",
            "schema": {
              "$ref": "#/definitions/v1ActivationCodeResponse"
            }
          }
        },
        "parameters": [
          {
            "name": "body",
            "in": "body",
            "required": true,
            "schema": {
              "$ref": "#/definitions/v1ActivationCodeRequest"
            }
          }
        ]
      }
    }
}
```

### /v1/api/credentials

Get access credentials for the platform.

```json
{
    "/v1/api/credentials": {
      "post": {
        "responses": {
          "200": {
            "description": "A successful response.",
            "schema": {
              "$ref": "#/definitions/v1CredentialsResponse"
            }
          }
        },
        "parameters": [
          {
            "name": "body",
            "in": "body",
            "required": true,
            "schema": {
              "$ref": "#/definitions/v1CredentialsRequest"
            }
          }
        ]
      }
    }
}
```

### /v1/api/credentials_renew

Renew a previously-issued access credential.

```json
{
    "/v1/api/credentials_renew": {
      "post": {
        "responses": {
          "200": {
            "description": "A successful response.",
            "schema": {
              "$ref": "#/definitions/v1CredentialsResponse"
            }
          }
        },
        "parameters": [
          {
            "name": "body",
            "in": "body",
            "required": true,
            "schema": {
              "$ref": "#/definitions/v1RenewCredentialsRequest"
            }
          }
        ]
      }
    }
}
```

### /v1/api/new_identifier

Helper method to generate a new DID instances for clients that can't
generate it locally. This is not recommended but supported for low end
devices and development purposes.

```json
{
    "/v1/api/new_identifier": {
      "post": {
        "responses": {
          "200": {
            "description": "A successful response.",
            "schema": {
              "$ref": "#/definitions/v1NewIdentifierResponse"
            }
          }
        },
        "parameters": [
          {
            "name": "body",
            "in": "body",
            "required": true,
            "schema": {
              "$ref": "#/definitions/v1NewIdentifierRequest"
            }
          }
        ]
      }
    }
}
```
