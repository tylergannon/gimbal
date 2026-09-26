package model

// DefaultThinkingLevel is pi's default reasoning level.
const DefaultThinkingLevel = ThinkingMedium

// ThinkingLevelOptions is the ordered set of thinking levels.
var ThinkingLevelOptions = []ThinkingLevel{
	ThinkingOff,
	ThinkingMinimal,
	ThinkingLow,
	ThinkingMedium,
	ThinkingHigh,
	ThinkingXHigh,
	ThinkingMax,
}
