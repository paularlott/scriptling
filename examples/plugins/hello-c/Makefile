CC      ?= gcc
CFLAGS  += -std=c11 -Wall -Wextra -O2
LDFLAGS += -lm -lpthread

TARGET  := hello-c
SRCS    := main.c scriptling_plugin.c

.PHONY: all clean test

all: $(TARGET)

$(TARGET): $(SRCS) scriptling_plugin.h
	$(CC) $(CFLAGS) -o $@ $(SRCS) $(LDFLAGS)

clean:
	rm -f $(TARGET)

test: all
	@mkdir -p /tmp/scriptling-plugins-c
	cp $(TARGET) /tmp/scriptling-plugins-c/$(TARGET)
	@echo "Built -> /tmp/scriptling-plugins-c/$(TARGET)"
	@echo ""
	@echo "Test commands:"
	@echo "  scriptling --plugin-dir /tmp/scriptling-plugins-c \\"
	@echo "    -c 'import plugin.hello; print(plugin.hello.greet(\"Ada\"))'"
	@echo ""
	@echo "  scriptling --plugin-dir /tmp/scriptling-plugins-c \\"
	@echo "    -c 'import plugin.hello; c = plugin.hello.Counter(10); c.inc(5); print(c.get(), c.value, c.label)'"
	@echo ""
	@echo "  scriptling --plugin-dir /tmp/scriptling-plugins-c \\"
	@echo "    -c 'import plugin.hello; events = []; print(plugin.hello.stream(lambda e: events.append(e))); print(events)'"
