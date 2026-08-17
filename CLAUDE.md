# Backend repo

## Important architecture rules
DRY and KISS are not jokes. Before implementing some new service/adapter/entity
or any other functionality I strongly suggest to see check if this one already
exists or at least place/object were this functionality can live exists.

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
Business layer. All service implementation goes there. All business rules like
validation, data transformation, data aggregation, algorithm orchestrastion
goes here.

Business layer is detached from outside world it only knows business. It has
transport layer to provide access for business logic to clients. And what is
more important in this layer is that it has abstraction called Adapter.

#### /internal/adapter
Adapter is the tool for business logic to access concrete database or concrete
cache. Clients for inter-service communication or accessing external 3rd party
APIs go there. Same with infra related cold storages, message brokers and etc.

#### /internal/entity
Entity is global data layer shared across all 3 layers. All DTOs, business models,
domain errors and etc should be declared there. Our goal here is proactive model
reusage and models strcucturing by file. This is really important because otherwise
this layer will become a total mess.

## Dependency defenition
Each time layer needs something that is not in its domain interface should be
declared.

## Errors
Every error declared on our side should be of type *cerr.Error. We must this and
only this due to its power of exstensibility and scalability.

Define domain errors in entity layer. Add kinds to it only if error itself sounds
like a 100% hit into this kind.

## Logging

All loggin should happen at transport layer. This will reduce number of pollution-like
logs and duplications. To not lose context we have defined in /pkg/cerr domain
error struct *Error. This is a robust way to define place where error was created,
its creation time, all needed context can be provided not only on creation time
but on the full stacktrace until processing. They could be wrapped so they will
carry all errors stack has.

EXCLUSION: the only log level that can be put everywhere is Debug.

## Comments

Don't use a lot of comments. Brevity is the soul of wit. Don't comment the code
because code itself should be really documenting itself. Use comments only at
components declarations like structs, interfaces, funcs and etc. But don't write
multi-line poems.
