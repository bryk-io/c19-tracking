# General Design

There are 4 main components:
- API Server: Receive and manage all user requests and handle administrative tasks.
- Storage: Provides persistent storage for data and handle queries.
- Message broker: Distribute messages in the system and allow for horizontal scaling.
- Client (mobile application): Track user location and dispatch events. Receive
  notifications from the API server. There can be several client applications.

## Security
Platform security is defined as privacy, authentication and authorization considerations.
In terms of privacy, no personally-identifiable information is ever used or collected.
For tasks requiring authentication mechanisms, anonymous bearer credentials are used in the
form of JWT - ES384 tokens. Finally, authorization requirements are managed using an RBAC
(i.e., Role-Based Access Control) model. 

There are 3 roles for users on the platform:
- Administrator: To manage and handle all assets on the platform.
- Agent: Agents represent official representatives of the health system and can generate
  notification requests. Based on the requirements for a particular setup (by country, state,
  city, etc), an agent can represent an individual hospital, an individual healthcare professional,
  etc.
- User: Use a client application to send location records and receive notifications.

## Instance Deployment.
- Setup an API server. As part of the installation process the API server will:
    - Connect to a storage provider.
    - Connect to a message broker to receive and handle async requests.
    - Generate a root CA certificate if not provided.
- Generate the credentials for an initial administrator.
- The administrator user will use the generated credentials to send authenticated requests
  to the API server.

## User On-Boarding.
- A user wishing to use the service installs a client application on his/her phone.
- When the user opens the application for the first time, the application informs the user it
  will track his location in the background and ask for the necessary permissions.
- The device generates and publish a new DID as device identifier.
- The DID private key is securely stored in the phone, and the identifier itself does not leak
  any identifiable details about the user. The user's privacy is protected.

## User Tracking.
- The application gets the user location and generates a record containing the following details:
    - Timestamp
    - GPS location
    - Device identifier
    - Digital signature
- The location record is send to the API server. The processing of a message is the following:
    - The message is placed on a processing queue.
    - The message is retrieved by a worker.
    - The format is validated.
    - The device identifier is verified, i.e., the DID is resolved and retrieved.
    - The digital signature is verified.
    - The location record is placed on the queue for storage.
    - When stored, the location record is indexed by its timestamp and location.
- The application will keep generating location records on the background every 60 seconds, or when
  the user's location change. Records that need to be reported will be locally stored if the device
  doesn't have an internet connection and send in batch once a connection is available. Each batch
  can include a maximum of 100 individual records.

## User Notification.
- When a user is tested and identified as a positive COVID-19 case by an official health
  care professional (i.e. the agent) a notification should be generated. 
- Using her application the agent scans the QR identifier on the user's phone. The application
  will generate a notification request with the following details:
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
- Valid notification are processed in the order they are placed in the queue (i.e., FIFO).
- The API server request a list of all identifiers that have been a meet record
  with the patient in the 3 weeks.
- The API server request a list of all identifiers that have been in proximity
  with the patient based on its ping records.
- The server dispatch notifications to all the users at risk using the available delivery
  mechanisms, for example: Push notifications, In-app notifications, etc.
