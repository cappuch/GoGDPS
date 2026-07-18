package gdlib

import "fmt"

// AudioTrack returns the built-in song name for an audio track ID.
func AudioTrack(id int) string {
	songs := []string{
		"Stereo Madness by ForeverBound", "Back on Track by DJVI", "Polargeist by Step",
		"Dry Out by DJVI", "Base after Base by DJVI", "Can't Let Go by DJVI",
		"Jumper by Waterflame", "Time Machine by Waterflame", "Cycles by DJVI",
		"xStep by DJVI", "Clutterfunk by Waterflame", "Theory of Everything by DJ Nate",
		"Electroman Adventures by Waterflame", "Club Step by DJ Nate", "Electrodynamix by DJ Nate",
		"Hexagon Force by Waterflame", "Blast Processing by Waterflame", "Theory of Everything 2 by DJ Nate",
		"Geometrical Dominator by Waterflame", "Deadlocked by F-777", "Fingerbang by MDK",
		"The Seven Seas by F-777", "Viking Arena by F-777", "Airborne Robots by F-777",
		"Secret by RobTopGames", "Payload by Dex Arson", "Beast Mode by Dex Arson",
		"Machina by Dex Arson", "Years by Dex Arson", "Frontlines by Dex Arson",
		"Space Pirates by Waterflame", "Striker by Waterflame", "Embers by Dex Arson",
		"Round 1 by Dex Arson", "Monster Dance Off by F-777",
	}
	if id < 0 || id >= len(songs) {
		return "Unknown by DJVI"
	}
	return songs[id]
}

func Difficulty(diff, auto, demon int) string {
	if auto != 0 {
		return "Auto"
	}
	if demon != 0 {
		return "Demon"
	}
	switch diff {
	case 0:
		return "N/A"
	case 10:
		return "Easy"
	case 20:
		return "Normal"
	case 30:
		return "Hard"
	case 40:
		return "Harder"
	case 50:
		return "Insane"
	default:
		return "Unknown"
	}
}

func DemonDiff(dmn int) string {
	switch dmn {
	case 3:
		return "Easy"
	case 4:
		return "Medium"
	case 5:
		return "Insane"
	case 6:
		return "Extreme"
	default:
		return "Hard"
	}
}

func LevelLength(length int) string {
	switch length {
	case 0:
		return "Tiny"
	case 1:
		return "Short"
	case 2:
		return "Medium"
	case 3:
		return "Long"
	case 4:
		return "XL"
	case 5:
		return "Platformer"
	default:
		return "Unknown"
	}
}

func GameVersion(version int) string {
	if version > 17 {
		return fmtFloat(float64(version) / 10)
	}
	switch version {
	case 11:
		return "1.8"
	case 10:
		return "1.7"
	default:
		return fmt.Sprintf("1.%d", version-1)
	}
}

func GauntletName(id int) string {
	gauntlets := []string{
		"Unknown", "Fire", "Ice", "Poison", "Shadow", "Lava", "Bonus", "Chaos", "Demon", "Time",
		"Crystal", "Magic", "Spike", "Monster", "Doom", "Death", "Forest", "Rune", "Force", "Spooky",
		"Dragon", "Water", "Haunted", "Acid", "Witch", "Power", "Potion", "Snake", "Toxic", "Halloween",
		"Treasure", "Ghost", "Spider", "Gem", "Inferno", "Portal", "Strange", "Fantasy", "Christmas",
		"Surprise", "Mystery", "Cursed", "Cyborg", "Castle", "Grave", "Temple", "World", "Galaxy",
		"Universe", "Discord", "Split",
	}
	if id < 0 || id >= len(gauntlets) {
		return gauntlets[0]
	}
	return gauntlets[id]
}

func fmtFloat(f float64) string {
	if f == float64(int(f)) {
		return fmt.Sprintf("%.1f", f)
	}
	return fmt.Sprintf("%g", f)
}
