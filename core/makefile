.DEFAULT_GOAL := install

BINARY_NAME=intunja
PREFIX?=/usr/local
BINDIR=$(PREFIX)/bin

.PHONY: install

build:
	go build -o $(BINARY_NAME)

install: build
	install -d $(DESTDIR)$(BINDIR)
	install -m 0755 $(BINARY_NAME) $(DESTDIR)$(BINDIR)/$(BINARY_NAME)

uninstall:
	rm -f $(DESTDIR)$(BINDIR)/$(BINARY_NAME)

clean:
	rm -f $(BINARY_NAME)