package fixspec

import "embed"

// Dictionaries contains standard QuickFIX definitions; FIX44.xml in the parent
// directory remains the user's separate business extension dictionary.
//
//go:embed FIX40.xml FIX41.xml FIX42.xml FIX43.xml standard/FIX44.xml
var Dictionaries embed.FS
