* unit tests
* Relook at implementation of AddLog()
* General refactoring and code cleanup & seperation
* Templating options for log output (that which is sent to logentries)
* Add option to parse Docker log JSON
* Tail: seek to last read position when reloading
* Change logger in in tail to our logger
* implement Reader/Writer interface where relevant
* reduce heavy use of globals
* Code comments