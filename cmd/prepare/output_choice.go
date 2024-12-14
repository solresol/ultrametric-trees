package prepare

type OutputChoice string

const (
    Paths OutputChoice = "paths"
    Words OutputChoice = "words"
    Hash  OutputChoice = "hash"
)

func IsValidOutputChoice(choice string) bool {
    switch OutputChoice(choice) {
    case Paths, Words, Hash:
        return true
    default:
        return false
    }
}
