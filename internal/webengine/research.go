package webengine

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Johnny008-creator/offcode/internal/memory"
)

// Adapted from internal/tui/research.go — see that file for the full
// rationale (reusing the existing agent loop via one crafted instruction
// message rather than a separate research engine).

var slugNonWord = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(s)
	s = slugNonWord.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 50 {
		s = s[:50]
		s = strings.Trim(s, "-")
	}
	if s == "" {
		s = "topic"
	}
	return s
}

func normalizeDepth(s string) string {
	switch strings.ToLower(s) {
	case "lite", "deep":
		return strings.ToLower(s)
	default:
		return "normal"
	}
}

// ResearchPrompt builds the depth-scoped instruction message for a
// /research-style request, returning the message to send to the model,
// the filename it's told to save to, and the normalized depth name.
func ResearchPrompt(question, depth string) (promptText, filename, depthName string) {
	depthName = normalizeDepth(depth)
	filename = fmt.Sprintf("research-%s-%s.md", slugify(question), time.Now().Format("2006-01-02"))

	var depthInstructions string
	switch depthName {
	case "lite":
		depthInstructions = `Depth: LITE. Do one web_search call, optionally fetch_url on the single most
relevant result if the search snippets aren't enough. Keep the final report short — a few paragraphs
or a tight bullet list is enough. Don't do more than one round of searching.`
	case "deep":
		depthInstructions = `Depth: DEEP. This should be a thorough investigation, not a quick lookup:
1. Start with plan_set breaking the question into the distinct angles/sub-questions it actually has.
2. Run multiple web_search calls from different angles (not just rephrasing the same query), and
   fetch_url on several of the best results per angle — don't settle for search snippets alone for
   anything that matters.
3. After the first round, explicitly notice what's still missing, contradictory, or under-sourced,
   and do at least one more round of targeted searches to fill those specific gaps.
4. Write a long-form final report: clear sections, specific facts attributed to sources, and a
   "Sources" list of the URLs you actually used at the end.
Don't stop after a single search round — the whole point of deep mode is the extra iterations.`
	default:
		depthInstructions = `Depth: NORMAL. Run several web_search calls from a few different angles (not
just one query), fetch_url on the handful of best results, and write a properly synthesized report
with a short "Sources" list of the URLs you used at the end — more than a one-paragraph summary, but
no need for multiple rounds of gap-filling searches.`
	}

	promptText = fmt.Sprintf(`Do deep research on the following question/topic, then save the result as a
markdown report.

Question: %s

%s

When the research itself is done, call write with path %q containing the final report (start it with
a "# " title line). After writing the file, give a short closing message confirming it's saved and
summarizing the top 2-3 findings inline — don't make the user open the file just to know the
headline answer.`, question, depthInstructions, filename)

	return promptText, filename, depthName
}

// RememberNote is the /api/remember handler's equivalent of /remember — a
// manual, reliable path into the same persistent memory store the
// remember meta-tool writes to.
func RememberNote(note string) error {
	return memory.Append(note)
}
