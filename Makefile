
run:
	DEBUG=1 go run . run haiku

install:
	go install


# Profiling:
# 1. import these in cmd/run.go: `"net/http"` and `_ "net/http/pprof"`
# 2. put this in RunTUI(): `go func() { log.Println(http.ListenAndServe("localhost:6060", nil)) }()`

pprof-cpu:
	go tool pprof -proto http://localhost:6060/debug/pprof/profile\?seconds\=8 > .pprof/cpu.prof

pprof-heap:
	go tool pprof -proto http://localhost:6060/debug/pprof/heap > .pprof/mem.prof

pprof-view-cpu:
	go tool pprof -http=:8080 .pprof/cpu.prof

pprof-view-mem:
	go tool pprof -http=:8080 .pprof/mem.prof
