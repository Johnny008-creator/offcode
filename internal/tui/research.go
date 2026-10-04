package tui

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Deep research mode (ROADMAP item 11) is deliberately NOT a separate
// meta-tool or nested sub-agent — it reuses the existing agent loop
// (web_search/fetch_url/plan_set/write) by handing it a single, carefully
// depth-scoped instruction as a normal user message. Simpler, and the
// existing loop is already capable enough that a parallel research engine
// would just be duplicated machinery.

type researchDepth int

const (
	researchLite researchDepth = iota
	researchNormal
	researchDeep
)

func parseResearchDepth(s string) (researchDepth, bool) {
	switch strings.ToLower(s) {
	case "lite":
		return researchLite, true
	case "normal":
		return researchNormal, true
	case "deep":
		return researchDeep, true
	}
	return researchNormal, false
}

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

// parseResearchCommand splits "/research <question> [--depth=lite|normal|deep]"
// into the question and the resolved depth. --depth can appear anywhere in
// the argument text.
func parseResearchCommand(text string) (question string, depth researchDepth) {
	depth = researchNormal
	words := strings.Fields(text)
	var kept []string
	for _, w := range words[1:] { // skip "/research" itself
		if strings.HasPrefix(w, "--depth=") {
			if d, ok := parseResearchDepth(strings.TrimPrefix(w, "--depth=")); ok {
				depth = d
			}
			continue
		}
		kept = append(kept, w)
	}
	return strings.Join(kept, " "), depth
}

func researchPrompt(question string, depth researchDepth) (promptText, filename string) {
	filename = fmt.Sprintf("research-%s-%s.md", slugify(question), time.Now().Format("2006-01-02"))

	var depthInstructions string
	switch depth {
	case researchLite:
		depthInstructions = `Depth: LITE. Do one web_search call, optionally fetch_url on the single most
relevant result if the search snippets aren't enough. Keep the final report short — a few paragraphs
or a tight bullet list is enough. Don't do more than one round of searching.`
	case researchDeep:
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

	return promptText, filename
}
