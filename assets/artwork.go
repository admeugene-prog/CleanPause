package assets

import _ "embed"

// CountdownArtwork preserves the supplied illustration and is included in the EXE.
//
//go:embed 6706b460-b36b-4687-bfff-7a07d08ffd6e.png
var CountdownArtwork []byte

//go:embed "Worried Boy and Sad Electronics.png"
var ReminderArtwork []byte

//go:embed инструкции.md
var Instructions string

//go:embed instructions.en.md
var EnglishInstructions string
