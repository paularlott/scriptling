# Records every HTMLParser callback for one input. Run unchanged by CPython
# (gen.py, to write golden.json) and by scriptling (htmlparser_test.go, to
# compare against it).
from html.parser import HTMLParser


class Recorder(HTMLParser):
    def handle_starttag(self, tag, attrs):
        self.events.append(["start", tag, [[n, v] for n, v in attrs], self.get_starttag_text()])

    def handle_endtag(self, tag):
        self.events.append(["end", tag])

    def handle_data(self, data):
        self.events.append(["data", data])

    def handle_comment(self, data):
        self.events.append(["comment", data])

    def handle_entityref(self, name):
        self.events.append(["entityref", name])

    def handle_charref(self, name):
        self.events.append(["charref", name])

    def handle_decl(self, decl):
        self.events.append(["decl", decl])

    def handle_pi(self, data):
        self.events.append(["pi", data])

    def unknown_decl(self, data):
        self.events.append(["unknown_decl", data])


def run(data, convert_charrefs):
    p = Recorder(convert_charrefs=convert_charrefs)
    p.events = []
    p.feed(data)
    p.close()
    return p.events
