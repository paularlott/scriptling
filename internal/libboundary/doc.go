// Package libboundary holds no code: its tests enforce the architecture of the
// library split so an embedding application links only the script libraries
// it registers.
//
// The tests run `go list` (and, without -short, `go build` and `go tool nm`)
// against the module and check:
//
//   - stdlib purity: the always-linked standard libraries import no
//     filesystem, network, process or host-access package and link only an
//     explicit allowlist of third-party modules;
//   - core purity: the root package and evaluator link the same small set of
//     third-party modules and nothing under extlibs/;
//   - no root extlibs package: extlibs/ holds only subdirectories;
//   - the leaf rule: an extension package does not import another extension
//     library package, except for the reviewed edges listed in the test;
//   - weight isolation: registering a light library (yaml, toml, ...) does not
//     drag in websocket, markdown, process-inspection, KV-store or HTTP code;
//   - an end-to-end link check that builds small embedders and inspects the
//     symbols of the resulting binaries.
//
// When one of these tests fails, its message names the rule that was broken
// and how to fix it. Change the tables in the tests only when the
// architecture itself is meant to change.
package libboundary
