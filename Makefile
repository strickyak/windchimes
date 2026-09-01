all: CHIMES

CHIMES:  BUILD-DIR   \
	build/windchimes.linux-amd64.exe \
	build/windchimes.linux-arm-7.exe \
	build/windchimes.linux-arm64.exe \
	build/windchimes.win-amd64.exe  \
	build/windchimes.win-386.exe    \
	build/windchimes.mac-arm64.exe  \
	build/windchimes.mac-amd64.exe

BUILD-DIR:
	mkdir -p build

build/windchimes.linux-amd64.exe: $(TETHER_GO_FILES)
	GOOS=linux   GOARCH=amd64       go build -o $@ .
build/windchimes.linux-386.exe: $(TETHER_GO_FILES)
	GOOS=linux   GOARCH=386         go build -o $@ .
build/windchimes.linux-arm-7.exe: $(TETHER_GO_FILES)
	GOOS=linux   GOARCH=arm GOARM=7 go build -o $@ .
build/windchimes.linux-arm64.exe: $(TETHER_GO_FILES)
	GOOS=linux   GOARCH=arm64       go build -o $@ .
build/windchimes.win-amd64.exe: $(TETHER_GO_FILES)
	GOOS=windows GOARCH=amd64       go build -o $@ .
build/windchimes.win-386.exe: $(TETHER_GO_FILES)
	GOOS=windows GOARCH=386         go build -o $@ .
build/windchimes.mac-arm64.exe: $(TETHER_GO_FILES)
	GOOS=darwin   GOARCH=arm64      go build -o $@ .
build/windchimes.mac-amd64.exe: $(TETHER_GO_FILES)
	GOOS=darwin   GOARCH=amd64      go build -o $@ .




