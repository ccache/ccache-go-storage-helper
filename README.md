# ccache-go-storage-helper

[ccache remote storage helpers] are implemented as standalone programs. This
module provides the common Go implementation of their IPC protocol and server.

[ccache remote storage helpers]: https://ccache.dev/storage-helpers.html

## Functionality

- ccache remote storage helper IPC protocol handling
- Unix socket and Windows named-pipe listeners
- Request dispatch for GET, PUT, REMOVE, EXISTS, INFO and STOP requests
- Common `CRSH_*` environment configuration parsing
- Idle timeout handling and diagnostic reporting
- Optional file logging

The protocol is specified in the [ccache remote storage helper specification].

[ccache remote storage helper specification]: https://github.com/ccache/ccache/blob/master/doc/remote_storage_helper_spec.md

## Using the library

```go
logger := storagehelper.NewLogger(os.Getenv("CRSH_LOGFILE"))
defer logger.Close()

config, err := storagehelper.ParseConfig(logger)
// ...
identity := "ccache-storage-example 1.0"
storage, err := newStorageClient(...) // implementation of the storagehelper.Storage interface
// ...
server := storagehelper.NewServer(config, identity, storage, logger)
server.Run()
```

## Development

```sh
make test
make format
```
