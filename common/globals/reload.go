package globals

var WebReloadChan = make(chan bool)
var MetricsReloadChan = make(chan bool)
var DatabaseReloadChan = make(chan bool)
var DatastoresReloadChan = make(chan bool)
var RecurringTasksReloadChan = make(chan bool)
var AccessTokenReloadChan = make(chan bool)
var CacheReplaceChan = make(chan bool)
var PluginReloadChan = make(chan bool)
