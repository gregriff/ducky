
run:
	DEBUG=1 go run . run haiku

install:
	go install


# Profiling:
# 1. import these in cmd/run.go: `"net/http"` and `_ "net/http/pprof"`
# 2. put this in RunTUI(): `go func() { log.Println(http.ListenAndServe("localhost:6060", nil)) }()`
#

PPROF_DIR = .pprof
PROFILE_DIR ?= ${PPROF_DIR}/$(shell date +"%m-%d_%H:%M")
PPROF_SECONDS ?= 10
PPROF_PORT ?= 8080

pprof-cpu:
	mkdir -p $(PROFILE_DIR)
	go tool pprof -proto http://localhost:6060/debug/pprof/profile\?seconds\=$(PPROF_SECONDS) > $(PROFILE_DIR)/cpu.prof

pprof-heap:
	go tool pprof -proto http://localhost:6060/debug/pprof/heap > $(PROFILE_DIR)/heap.prof

# Opens an existing pprof file in the web browser
pprof-view:
	@echo "Viewing \$PPROF_FILE: $${PPROF_FILE:?PPROF_FILE is not set}"
	go tool pprof -http=:$(PPROF_PORT) $${PPROF_FILE}


.PHONY: run install pprof-cpu pprof-heap pprof-view-cpu pprof-view-mem
