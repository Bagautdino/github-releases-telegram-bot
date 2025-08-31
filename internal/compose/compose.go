package compose

import (
	"fmt"
	"html"
	"regexp"
	"strings"
	"time"
)

var bulletRe = regexp.MustCompile(`(?m)^\s*[-*•]\s+(.+)$`)

// Options for composing messages
type Options struct {
	MaxBullets int
	MaxChars   int
	TimeZone   string
}

// Input data for composing a message
type Input struct {
	RepoFull  string
	Tag       string
	URL       string
	BodyMD    string
	Published time.Time
	Advisor   string // optional LLM advice
}

// BuildHTML creates an HTML-formatted message for Telegram
func BuildHTML(in Input, opt Options) string {
	loc, _ := time.LoadLocation(opt.TimeZone)
	if loc == nil {
		loc = time.UTC
	}

	date := in.Published.In(loc).Format("2006-01-02 15:04")
	bullets := TakeBullets(in.BodyMD, opt.MaxBullets, opt.MaxChars)

	var sb strings.Builder
	// Более компактный заголовок
	sb.WriteString("🔥 <b>")
	sb.WriteString(html.EscapeString(in.RepoFull))
	sb.WriteString("</b> ")
	sb.WriteString(`<a href="` + in.URL + `">` + html.EscapeString(in.Tag) + "</a>\n")
	
	// Дата в одну строку с меньшими отступами
	sb.WriteString("📅 " + date + "\n")

	// Более компактные буллеты (максимум 4 для читаемости)
	if len(bullets) > 0 {
		maxToShow := min(len(bullets), 4)
		for i := 0; i < maxToShow; i++ {
			if bullets[i] != "" {
				sb.WriteString("\n▪️ " + bullets[i])
			}
		}
		if len(bullets) > 4 {
			sb.WriteString(fmt.Sprintf("\n<i>... и ещё %d изменений</i>", len(bullets)-4))
		}
		sb.WriteString("\n")
	}

	// Ссылка на changelog
	sb.WriteString(`\n<a href="` + in.URL + `">📖 Полный changelog</a>`)

	// LLM совет более компактно
	if strings.TrimSpace(in.Advisor) != "" {
		sb.WriteString("\n\n💡 " + html.EscapeString(in.Advisor))
	}

	return sb.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TakeBullets extracts bullet points from markdown text
func TakeBullets(md string, maxBullets, maxChars int) []string {
	// First try to find bullet points
	lines := bulletRe.FindAllStringSubmatch(md, -1)
	var bullets []string

	for _, match := range lines {
		if len(match) < 2 {
			continue
		}

		bullet := strings.TrimSpace(match[1])
		bullet = stripFormatting(bullet)

		// Skip technical noise
		if isSkippableBullet(bullet) {
			continue
		}

		// Limit bullet length
		if len(bullet) > 140 {
			bullet = bullet[:140] + "…"
		}

		if bullet != "" && len(bullet) > 10 {
			bullets = append(bullets, bullet)
			if len(bullets) >= maxBullets {
				break
			}
		}
	}

	// If no bullets found, extract from paragraph text
	if len(bullets) == 0 {
		bullets = extractParagraphs(md, maxBullets, maxChars)
	}

	return bullets
}

// isSkippableBullet filters out technical noise from changelog
func isSkippableBullet(bullet string) bool {
	bullet = strings.ToLower(bullet)
	
	// Skip if contains hashes (SHA, checksums)
	if strings.Contains(bullet, "sha") || regexp.MustCompile(`[a-f0-9]{8,}`).MatchString(bullet) {
		return true
	}
	
	// Skip version bumps and dependency updates (unless major)
	if strings.Contains(bullet, "bump") || strings.Contains(bullet, "update") {
		if strings.Contains(bullet, "version") || strings.Contains(bullet, "dependency") {
			return true
		}
	}
	
	// Skip file names and technical files
	if strings.Contains(bullet, ".zip") || strings.Contains(bullet, ".tar.gz") || 
	   strings.Contains(bullet, ".exe") || strings.Contains(bullet, "<!-- ") {
		return true
	}
	
	// Skip very short or very long bullets
	if len(bullet) < 15 || len(bullet) > 200 {
		return true
	}
	
	return false
}

// extractParagraphs extracts meaningful paragraphs when no bullets are found
func extractParagraphs(md string, maxBullets, maxChars int) []string {
	text := stripFormatting(md)

	// Limit total text length
	if len(text) > maxChars {
		text = text[:maxChars] + "…"
	}

	// Split into paragraphs
	paragraphs := strings.Split(text, "\n")
	var bullets []string

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)

		// Skip empty lines and very short lines
		if len(para) < 10 {
			continue
		}

		// Skip lines that look like headers (all caps, or starting with #)
		if strings.HasPrefix(para, "#") || isAllCaps(para) {
			continue
		}

		// Limit paragraph length
		if len(para) > 200 {
			para = para[:200] + "…"
		}

		bullets = append(bullets, para)
		if len(bullets) >= maxBullets {
			break
		}
	}

	return bullets
}

// stripFormatting removes markdown formatting and escapes HTML
func stripFormatting(s string) string {
	// Remove common markdown syntax
	s = strings.ReplaceAll(s, "`", "")
	s = regexp.MustCompile(`[_*~#>]+`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`\[([^\]]+)\]\([^)]+\)`).ReplaceAllString(s, "$1")  // Links
	s = regexp.MustCompile(`!\[([^\]]*)\]\([^)]+\)`).ReplaceAllString(s, "$1") // Images

	// Clean up whitespace
	s = regexp.MustCompile(`\s+`).ReplaceAllString(s, " ")
	s = strings.TrimSpace(s)

	// Escape HTML
	s = html.EscapeString(s)

	return s
}

// isAllCaps checks if string is mostly uppercase (likely a header)
func isAllCaps(s string) bool {
	if len(s) < 3 {
		return false
	}

	upperCount := 0
	letterCount := 0

	for _, r := range s {
		if r >= 'A' && r <= 'Z' {
			upperCount++
			letterCount++
		} else if r >= 'a' && r <= 'z' {
			letterCount++
		}
	}

	if letterCount == 0 {
		return false
	}

	// Consider it "all caps" if 80%+ of letters are uppercase
	return float64(upperCount)/float64(letterCount) > 0.8
}
