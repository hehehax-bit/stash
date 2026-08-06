package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// SessionIntent is the parsed result of a natural-language session request.
type SessionIntent struct {
	DurationMinutes int      `json:"duration_minutes"`
	Moods           []string `json:"moods"`
	Ordering        string   `json:"ordering"`
	MinSteam        int      `json:"min_steam"`
	Vibe            string   `json:"vibe"`
}

const validOrderings = "build_up, peak_first, or empty"

// ParseSessionIntent asks the model to turn a free-text session request into
// structured parameters.
func ParseSessionIntent(ctx context.Context, client *Client, text string) (SessionIntent, error) {
	systemPrompt := "You are a session planner for an adult media library. Translate the user's request into structured JSON only."
	userPrompt := fmt.Sprintf(`Parse this session request into JSON with these fields:
- "duration_minutes": integer
- "moods": array of mood strings from: romantic, rough, goth, cosplay, amateur, milf, bdsm, anal, threesome, taboo, hardcore, sensual, humor, solo, lesbian, gangbang, cuckold, dirty talk (empty array if none mentioned; for sequences keep them in the given order)
- "ordering": one of %s ("build_up" means ending with the most intense)
- "min_steam": integer 1-10 (0 if unknown)
- "vibe": a short phrase capturing the mood/atmosphere for embedding search ("" if none)

Request: %s

Return ONLY valid JSON, no other text.`, validOrderings, text)

	messages := []ChatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	resp, err := client.ChatCompletion(ctx, ChatCompletionRequest{Messages: messages})
	if err != nil {
		return SessionIntent{}, fmt.Errorf("session intent request failed: %w", err)
	}

	content, ok := resp.Choices[0].Message.Content.(string)
	if !ok {
		return SessionIntent{}, fmt.Errorf("unexpected content type from AI response")
	}

	cleanJSON := ExtractJSON(content)
	if cleanJSON == "" {
		return SessionIntent{}, fmt.Errorf("parsing AI response: no JSON found")
	}

	var intent SessionIntent
	if err := json.Unmarshal([]byte(cleanJSON), &intent); err != nil {
		return SessionIntent{}, fmt.Errorf("parsing AI response: %w", err)
	}
	if intent.DurationMinutes <= 0 {
		intent.DurationMinutes = 30
	}
	if intent.MinSteam < 0 || intent.MinSteam > 10 {
		intent.MinSteam = 0
	}
	intent.Ordering = strings.TrimSpace(intent.Ordering)

	return intent, nil
}
