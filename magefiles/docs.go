//go:build mage

// Documentation targets — assemble derived doc artifacts from primary sources.
package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
)

// DocsStandalone regenerates docs/design-system-standalone.html by inlining
// docs/botctrl-docs.css and docs/botctrl-docs.js into docs/design-system.html.
//
// The standalone file is a shareable single-file edition of the design-system
// guide. Run this after changing the design system's CSS, JS, or the source
// design-system.html so the standalone copy stays in sync.
func DocsStandalone(ctx context.Context) error {
	const (
		srcHTML  = "docs/design-system.html"
		srcCSS   = "docs/botctrl-docs.css"
		srcJS    = "docs/botctrl-docs.js"
		dstHTML  = "docs/design-system-standalone.html"
		linkLine = `<link rel="stylesheet" href="botctrl-docs.css">`
		scriptLn = `<script src="botctrl-docs.js"></script>`
	)

	html, err := os.ReadFile(srcHTML)
	if err != nil {
		return fmt.Errorf("reading %s: %w", srcHTML, err)
	}
	css, err := os.ReadFile(srcCSS)
	if err != nil {
		return fmt.Errorf("reading %s: %w", srcCSS, err)
	}
	js, err := os.ReadFile(srcJS)
	if err != nil {
		return fmt.Errorf("reading %s: %w", srcJS, err)
	}

	cssBlock := "<style>\n" + string(bytes.TrimSpace(css)) + "\n</style>"
	jsBlock := "<script>\n" + string(bytes.TrimSpace(js)) + "\n</script>"

	out := string(html)
	out = strings.Replace(out, linkLine, cssBlock, 1)
	out = strings.Replace(out, scriptLn, jsBlock, 1)

	// Standalone-only edits: title, rail labels, hero badges, footer, and a
	// callout near §01 explaining what this file is.
	standaloneEdits := []struct{ from, to string }{
		{
			`<title>Docs Design System — botctrl</title>`,
			`<title>Docs Design System (Standalone) — botctrl</title>`,
		},
		{
			`<p class="rail-title">// docs · design system</p>
    <p class="rail-product"><span class="live-dot" aria-hidden="true"></span>Docs Design System</p>`,
			`<p class="rail-title">// docs · standalone</p>
    <p class="rail-product"><span class="live-dot" aria-hidden="true"></span>Design System</p>`,
		},
		{
			`<span class="badge primary">v1 guide</span>
        <span class="badge tests">portable</span>
        <span class="badge swift">html · css · js</span>`,
			`<span class="badge primary">v1 guide</span>
        <span class="badge tests">standalone</span>
        <span class="badge swift">single file</span>`,
		},
		{
			`<li>Copy one existing page (e.g. <a href="kong-base-setup.html">kong-base-setup.html</a>) as a template and overwrite the content.</li>`,
			`<li>Copy one existing page (e.g. <code>kong-base-setup.html</code> from the full botctrl docs folder) as a template and overwrite the content.</li>`,
		},
		{
			`<section id="what-this-is">
      <h2><span class="section-anchor">// 01</span>What This Is</h2>

      <p>The botctrl docs design system`,
			`<section id="what-this-is">
      <h2><span class="section-anchor">// 01</span>What This Is</h2>

      <div class="callout note"><strong>note</strong>This is the <em>single-file</em> edition of the docs design system guide — HTML, CSS, and JS bundled into one file you can save, send, or open offline without any other assets. The CSS and JS live in <code>&lt;style&gt;</code> and <code>&lt;script&gt;</code> blocks inside <code>&lt;head&gt;</code> (necessary for the theme toggle to set the right colour before first paint; putting them at the bottom of the file would cause a flash of unstyled content). The "normal" edition splits them into <code>botctrl-docs.css</code> and <code>botctrl-docs.js</code> so multiple pages share one cache entry — see <code>docs/design-system.html</code> in the botctrl repository.</div>

      <p>The botctrl docs design system`,
		},
		{
			`<footer>
      <span class="blink">botctrl</span>
      <span>design-system · docs</span>
    </footer>`,
			`<footer>
      <span class="blink">botctrl</span>
      <span>design-system · standalone · single file</span>
    </footer>`,
		},
	}

	for _, edit := range standaloneEdits {
		next := strings.Replace(out, edit.from, edit.to, 1)
		if next == out {
			return fmt.Errorf("standalone edit failed to match: %q...", edit.from[:min(60, len(edit.from))])
		}
		out = next
	}

	if err := os.WriteFile(dstHTML, []byte(out), 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", dstHTML, err)
	}
	fmt.Printf("wrote %s (%d bytes)\n", dstHTML, len(out))
	return nil
}
