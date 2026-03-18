package manpage

import _ "embed"

//go:generate cp ../../sortie.1 sortie.1

// Content holds the roff-formatted manual page, embedded at build time.
//
//go:embed sortie.1
var Content string
