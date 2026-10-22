# Regenerates the html.parser reference data from CPython. Run from the
# repository root with Python 3.14:
#
#   python3.14 -I extlibs/testdata/htmlparser/gen.py
#
# It writes extlibs/htmlparser_entities.go (the HTML5 named character
# references html.unescape uses) and extlibs/testdata/htmlparser/golden.json
# (CPython's callbacks for every corpus input, with and without
# convert_charrefs, and for seeded random markup).
#
# The corpus leaves out the raw-text elements (script, style, xmp, iframe,
# noembed, noframes, plaintext, textarea, title), whose content scriptling
# does not yet treat as raw text, and inputs split across several feed()
# calls, which scriptling does not buffer.
import html.entities
import json
import os
import random
import runpy
import subprocess
import sys

if sys.version_info[:2] != (3, 14):
    sys.exit("gen.py must be run with Python 3.14")

here = os.path.dirname(os.path.abspath(__file__))
extlibs = os.path.dirname(os.path.dirname(here))

CORPUS = [
    # ordinary documents
    "<html><body><p>Hello</p></body></html>",
    '<!DOCTYPE html><html lang="en"><head><meta charset="utf-8"></head><body class="x y">Hi</body></html>',
    '<a href="https://example.com" class="link">Link</a>',
    "<ul>\n  <li>one</li>\n  <li>two</li>\n</ul>\n",
    '<p>a<b>c</p>',
    '<DIV CLASS="X" Id=y>Text</DIV>',
    "<br><br/><br /><img src=x.png /><hr/>",
    "<input disabled type=checkbox checked>",
    'héllo <b>wörld</b> &eacute; ☃ \U0001F600',
    # attributes
    '<a href="x" href="y">',
    '<a title="x>y">',
    "<a title='say \"hi\"'>",
    '<a title="it\'s">',
    "<a b=c d>",
    "<a b=>",
    "<a b = 'c' >",
    "<a\nb='c'\td=\"e\"\fg=h>",
    "<a =b>",
    "<a/b>",
    "<a/>",
    "<a />",
    "<a/ >",
    "<a b/>",
    "<a b=c/>",
    "<a b='c'/>",
    "<a href=/x/>",
    "<a b=\"c\"d=\"e\">",
    "<a b='c\"d'>",
    '<a b=c"d>',
    "<a b==c>",
    "<a \x00b=c>",
    '<a href="?x=1&amp;y=2&ampz=3&amp=4&#65;&#x42;&notit;&notin;&lt">',
    '<a v="&amp;&AMP;&amp&lt;&LT&gt;&quot;&nbsp;&copy;&copy">',
    '<a v="&#0;&#x80;&#1;&#xFFFE;&#xD800;&#x110000;&#13;">',
    '<a v="&unknown;&a;&;&#;&#x;">',
    # end tags
    "</a>",
    "</a >",
    "</ a>",
    "</a b>",
    "</a b='c'>",
    "</A>",
    "</1>",
    "</>",
    "</ >",
    "<//a>",
    "</a/>",
    "</!x>",
    # malformed start tags
    "<>",
    "< >",
    "<1>",
    "< a>",
    "<a<b>",
    "<<a>>",
    "<a><<",
    "a<",
    "<",
    "x</",
    "</",
    "<a",
    '<a b="c',
    "<a b='c",
    "<a b=c",
    "<a b",
    "</a",
    "</a b",
    # comments
    "<!---->",
    "<!--x-->",
    "<!-- a -- b -->",
    "<!--x--!>",
    "<!-->",
    "<!--->",
    "<!--",
    "<!--x",
    "<!--x-",
    "<!--x--",
    "<!--x--!",
    "<!--a--><!--b-->",
    "<!--<a>-->",
    "<!--->x-->",
    # declarations, bogus comments, processing instructions, CDATA
    "<!DOCTYPE html>",
    "<!doctype html>",
    "<!DocType html PUBLIC \"-//W3C//DTD HTML 4.01//EN\">",
    "<!doctype html",
    "<!>",
    "<!x>",
    "<!x",
    "<!",
    "<!-x>",
    "<![CDATA[x<y]]>",
    "<![CDATA[x",
    "<![cdata[x]]>",
    "<?xml version=\"1.0\"?>",
    "<?>",
    "<?",
    "<?x",
    "<?php echo 1; ?>",
    # character references in text
    "a &; b",
    "&",
    "x&",
    "&#",
    "&#;",
    "&#x;",
    "&#x",
    "&#xg",
    "&#a",
    "a & b",
    "a &b c",
    "&amp;&lt;&gt;",
    "x &amp; y",
    "&amp",
    "x &am",
    "&amp x",
    "&ampx",
    "&notit;",
    "&notin;",
    "&#65;&#x41;&#65",
    "&#65 x",
    "&#x41g",
    "&#x41;&#X41;",
    "&#0;&#x80;&#x81;&#1;&#xb;&#x7f;&#xFFFE;&#xD800;&#x110000;&#99999999999;&#13;",
    "&a-b;",
    "&amp ;",
    "&é;",
    "&zwnj;",
    "&LT;&lt",
    "&unknown;",
    "AT&T",
    "<p>&lt;tag&gt; &amp;&amp; &#8364;</p>",
    "a\r\nb\rc",
    "\t\n\f ",
    "",
    # "&#" lookahead counts characters, not bytes
    "x&#\U0001F600",
    "x&#\u2603",
    "x&#\u2603y",
]

# Random markup fragments, seeded so the golden data is stable.
FRAGMENTS = [
    "<", ">", "/", "</", "<!", "<!--", "-->", "--!>", "-", "--", "!", "?", "<?", "[", "]]>", "<![CDATA[",
    "&", "&#", "&#x", ";", "=", '"', "'", " ", "\t", "\n", "\r", "\f", "\x00", "a", "b", "div", "A", "B", "x",
    "1", "9", "f", "F", "amp", "lt", "gt", "notit", "notin", "nbsp", "AMP", "&amp", "&lt;", "&#65", "&#x41",
    "doctype", "DOCTYPE", "<!doctype", "href", "class", "=x", '="y"', "='z'", "\u00e9", "\u2603", "\U0001F600",
    ".", "-x", "0", "80", "FFFE", "D800", "110000", " />", "/>", "<a", "<b ", "</a>", "<br/>", "\u00e9;",
]
RAW_TEXT = ("script", "style", "xmp", "iframe", "noembed", "noframes", "plaintext", "textarea", "title")
rng = random.Random(20261022)
random_inputs = []
while len(random_inputs) < 1500:
    data = "".join(rng.choice(FRAGMENTS) for _ in range(rng.randint(1, 14)))
    if not any("<" + t in data.lower() for t in RAW_TEXT):
        random_inputs.append(data)

harness = runpy.run_path(os.path.join(here, "harness.py"))
golden = []
for data in CORPUS:
    for cc in (True, False):
        golden.append({"input": data, "convert_charrefs": cc, "events": harness["run"](data, cc)})
for n, data in enumerate(random_inputs):
    cc = n % 2 == 0
    golden.append({"input": data, "convert_charrefs": cc, "events": harness["run"](data, cc)})
with open(os.path.join(here, "golden.json"), "w", encoding="utf-8") as f:
    json.dump(golden, f, ensure_ascii=True, indent=0)
    f.write("\n")


def go_quote(s):
    out = []
    for ch in s:
        o = ord(ch)
        if ch in '"\\':
            out.append("\\" + ch)
        elif 0x20 <= o < 0x7F:
            out.append(ch)
        elif o <= 0xFFFF:
            out.append("\\u%04x" % o)
        else:
            out.append("\\U%08x" % o)
    return '"' + "".join(out) + '"'


lines = [
    "// Code generated by extlibs/testdata/htmlparser/gen.py from CPython %d.%d.%d; DO NOT EDIT." % sys.version_info[:3],
    "",
    "package extlibs",
    "",
    "// html5Entities are the HTML5 named character references, keyed as in",
    "// Python's html.entities.html5: with their trailing semicolon, and also",
    "// without it for the legacy names that may omit it.",
    "var html5Entities = map[string]string{",
]
for name in sorted(html.entities.html5):
    lines.append("\t%s: %s," % (go_quote(name), go_quote(html.entities.html5[name])))
lines.append("}")
lines.append("")
entities_go = os.path.join(extlibs, "htmlparser_entities.go")
with open(entities_go, "w", encoding="utf-8") as f:
    f.write("\n".join(lines))
subprocess.run(["gofmt", "-w", entities_go], check=True)
print("wrote %d golden cases and %d entities" % (len(golden), len(html.entities.html5)))
