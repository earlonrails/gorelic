GLIDE_GO_EXECUTABLE ?= go
DIST_DIRS := find * -type d -exec

deps: glide
	./glide install

glide:
ifeq ($(shell uname),Darwin)
	curl -L https://github.com/Masterminds/glide/releases/download/0.10.2/glide-0.10.2-darwin-amd64.zip -o glide.zip
	unzip glide.zip
	mv ./darwin-amd64/glide ./glide
	rm -fr ./darwin-amd64
	rm ./glide.zip
else

	curl -L https://github.com/Masterminds/glide/releases/download/0.10.2/glide-0.10.2-linux-386.zip -o glide.zip
	unzip glide.zip
	mv ./linux-386/glide ./glide
	rm -fr ./linux-386
	rm ./glide.zip
endif

test:
	${GLIDE_GO_EXECUTABLE} test -v .

clean:
	rm ./glide

.PHONY: test