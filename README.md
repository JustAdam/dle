DLE (docker logentries)
-----------------------

A client to send your Docker logs to logentries.com.

## First steps

- Configure all your containers to log to stdout/stderr
- Set up log rotation on your Docker logs

## Next

### Configuration

Configuration file is in toml format (config.toml).

- Open up config.toml
- Add any container IDs you wish to ignore too `ignore = []`
- All containers can either be logged to a single log, or each container to a seperate log.
- Log into your account and logentries.
- Add a new host and select manual configuration.

#### Single log

- Give your log a relevant name
- Select Token TCP
- Registry new log
- Copy the token given, and pass it to dle either by the environment variable `DLE_DEFAULT_LOG_KEY` or flag `--default-log-key`

#### Log per container

You need to know each container ID you want to create a log for.  Container names are not currently supported.

- Follow the steps above and update the config.toml file with a similar entry:

```
[container.CONTAINER-ID]
key = "LOGENTRIES-TOKEN"
```
- Repeat for each container

## Building

- Check out this repository
- Install gvp and gpm (used for package management)

```
$ gvp init
$ source gvp in
$ gpm install
$ go build -ldflags="-s" -o dle dle.go tls.go
$ mv dle /usr/bin/dle
```

## Docker image

```
$ docker pull justadam/dle
$ docker run -d --name dle -e DLE_DEFAULT_LOG_KEY=YOUR_DEFAULT_TOKEN -e DLE_LOG_DIRECTORY=/docker-containers -e DLE_LOG_LEVEL=fatal -e DLE_WATCH_LOG_DIRECTORY=true -v /var/lib/docker/containers/:/docker-containers -v /path/to/where/config/file/is:/etc/dle/ justadam/dle:latest
```

After starting you will probably want to ignore this docker container; so add the container ID to `ignore` in config.toml and reload dle's configuration:

```
$ docker kill -s HUP dle
```