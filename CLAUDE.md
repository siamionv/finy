# Backend repo

## Project structure

### /cmd/<bin>/main.go
Application entry points. Contains application dependency wire grapgh.

### /configs
Place for service configuration for different environments:
- local.yaml - local environment
- prod.yaml - production environment
- ...

### /docs
Place for API contracts. Proto and openapi files should be here

### /internal
This is the place for code that is supposed to be used only in service boundaries.

#### /internal/config
Place where config is parsed and code bindings are declared.

#### /internal/di
Place where service dependencies like clients, db pools and etc are created.

#### /internal/generated/<tool>
Directory that contain generated code (e.g. oapigen, sqlc and etc).

#### /internal/handler
Transport layer. Directories inside should be firstly divided by transport type:
- httpapi
- grpcapi
...

Inside transport type follow best pracitces. Group by API versioning, domain and etc.

This layer contains only:
- Defines its interface dependencies on business layer
- Reading client request
- Adopting the request in data models suitable for business layer
- Passing data into business layer
- Adopting results of business layer processing (handling defined-error cases)
- Writing client response

IMPORTANT: transport layer is not and orchestrator of business logic. It must
call only one business method at a time. Its business layer responsibility to jungle
business rules and etc.

#### /internal/business
Business layer. All service implementation goes there

Inside transport type follow best pracitces. Group by API versioning, domain and etc.

This layer contains only:
- Reading client request
- Adopting the request in data models suitable for business layer
- Passing data into business layer
- Adopting results of business layer processing (handling defined-error cases)

#### /internal/adapter

## Logging

All loggin should happen at transport layer. This will reduce number of pollution-like
logs and duplications. To not lose context we have defined in /pkg/cerr domain
error struct *Error. This is a robust way to define place where error was created,
its creation time, all needed context can be provided not only on creation time
but on the full stacktrace until processing. They could be wrapped so they will
carry all errors stack has.

EXCLUSION: the only log level that can be put everywhere is Debug.
