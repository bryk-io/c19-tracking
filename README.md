# General Design

There are 4 main components:
- Storage: Provides persistent storage for data and handle queries.
- Message broker: Distribute messages in the system.
- API Server: Receive requests and handle administrative tasks.
- Client (mobile application): Track user location and dispatch events. Receive notifications from
  the API server.

There are 3 types of users on the platform:
- Administrator: Can register new namespaces in the system.
- Agent: An agent is assigned to a namespace and can generate notification requests. An
  agent can be the complete health system (by country, state, city, etc), an individual hospital,
  an individual healthcare professional, etc.
- User: Use a client application to send location records and receive notifications.

## Instance Deployment.
- Setup an API server. As part of the installation process the API server will:
    - Connect to a storage provider.
    - Connect to a message broker to receive and handle async requests.
    - Generate a root CA certificate if not provided.
    - Generate the credentials for an initial administrator user if required.
- The administrator user will use the generated credentials to send authenticated requests
  to the API server. 
- The first task is to register a namespace, for example "MXN". Users, records, events
  and notifications are managed at a per-namespace level. An intermediate CA is generated
  for each namespace.
- At least one agent should be registered for each namespace.

## User On-Boarding.
- A user wishing to use the service installs a client application on this phone.
- When the user opens the application for the first time it will ask the user to enter the
  configuration details: API server address and, his selected namespace. This can be simplified
  by scanning a QR code or selecting a country flag from a list for example.
- The application informs the user it will track his location in the background and ask
  for the necessary permissions.
- The device generates and publish a new DID as device identifier.
- The DID private key is securely stored in the phone, and the identifier itself does not leak
  any identifiable details about the user. The user's privacy is protected.

## User Tracking.
- The application gets the user location and build a ping record containing the following details:
    - Timestamp
    - GPS location
    - Device identifier
    - Digital signature
- The ping record is send to the API server. The processing of a message is the following:
    - The message is placed on a processing queue.
    - The message is retrieved by a worker.
    - The format is validated.
    - The device identifier is verified, i.e., the DID is resolved and retrieved.
    - The digital signature is verified.
    - The ping record is placed on the queue for storage.
    - When stored the ping message is indexed by its timestamp and location.
- The application will keep generating ping records on the background every 15 seconds. If the
  user location has not varied more than 15 meters from its last reported location the record is
  discarded. Records that need to be reported will be locally stored if the device doesn't have
  an internet connection and send in batch once a connection is available. Each batch can include
  a maximum of 100 records.

## Meet Tracking.
- On devices were the bluetooth connection is available, the device will listen for new peers
  and attempt to exchange identifiers when one is detected. The event is registered locally and
  send to the API server as a "meet" record.
- The meet record contains the following details:
    - Timestamp
    - GPS location
    - Device identifier
    - Peer identifier
    - Meet code (MC, randomly generated)
    - "MC" signed by the device
    - "MC" signed by the peer
Meet records that need to be reported will be locally stored if the device doesn't have
an internet connection and send in batch once a connection is available. Each batch can
include a maximum of 100 records.

## User Notification.
- When a user is tested and identified as a positive COVID-19 case by an official health
  care professional (i.e. the agent) a notification should be generated. 
- Using her application the agent scans the QR identifier on the user's phone. The application
  will generate a notification request with the following details:
    - Timestamp
    - GPS location
    - Patient's device identifier
    - Agent identifier and certificate
    - Digital signature
- The notification request is send to the API server for validation.
    - Validate request format
    - Validate agent credentials
    - Validate digital signature in the request is valid
    - Validate a notification has not been already dispatched for the same patient
    - Place the notification in the queue for processing
- Valid notification are processed in the order they are placed in the queue (i.e., FIFO).
    - The API server request a list of all identifiers that have been a meet record
      with the patient in the 3 weeks.
    - The API server request a list of all identifiers that have been in proximity
      with the patient based on its ping records.
    - The server dispatch notifications to all the users at risk using the channels available.
- Notifications are delivered via channels enabled. For example:
    - Push notifications
    - In-app notifications

## API Overview
/api/v1/register_namespace
/api/v1/register_ping
/api/v1/register_ping_batch
/api/v1/register_meet
/api/v1/register_meet_batch
/api/v1/dispatch_notification
