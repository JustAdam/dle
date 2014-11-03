* unit tests
* Relook at implementation of AddLog()
* General refactoring, code cleanup & seperation
* Templating options for log output (String methods / that which is sent to logentries)
* Tail: seek to last read position when reloading config file
* Change logger in tail to our logger
* implement Reader/Writer interface where relevant (watchLogs should implement io.Reader)
* reduce heavy use of globals
* Code commenting