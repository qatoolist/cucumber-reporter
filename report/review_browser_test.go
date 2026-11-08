//go:build browser

package report

import "testing"

// These interaction regressions run only in the browser release gate.
func TestBrowserReviewRegressions(t *testing.T) {
	features, opt := loadShowcase(t, "")
	s := openSession(t, features, opt, true)
	s.goTo("/scenarios?layout=table&tableSort=name&tableDir=desc")
	s.eval(`window.reviewOrder=Array.from(document.querySelectorAll('.scenario-table tbody a'),a=>a.getAttribute('href'));document.querySelector('.scenario-table tbody a').click()`, nil)
	s.settle()
	s.must(`document.querySelector('.pn a:last-child').getAttribute('href')===window.reviewOrder[1]`)
	s.goTo("/features/order-processing")
	s.must(`document.querySelector('[aria-label="Saved view name"]')`)
	s.eval(`document.querySelector('[aria-label="Saved view name"]').value='Feature scope';Array.from(document.querySelectorAll('button')).find(x=>x.textContent==='Save view').click()`, nil)
	s.must(`new URLSearchParams(JSON.parse(localStorage.getItem('cucumber-reporter:saved-views:v1')).find(x=>x.name==='Feature scope').query).get('feature')==='order-processing'`)
	s.eval(`document.querySelector('[aria-label="Print or save as PDF"]').click();document.querySelector('[aria-label="Print scope"]').value='failures';Array.from(document.querySelectorAll('dialog button')).find(x=>x.textContent==='Preview').click()`, nil)
	s.settle()
	s.must(`document.querySelector('.pr-unexpected').getClientRects().length>0 && document.querySelector('.pr-unexpected').parentElement.hidden===false`)
	s.must(`Array.from(document.querySelectorAll('.pr-sc')).filter(x=>!x.hidden).length===5`)
}

func TestBrowserRound14StepAndEvidenceControls(t *testing.T) {
	features, opt := loadShowcase(t, "")
	s := openSession(t, features, opt, true)
	s.goTo("/scenarios/invoicing-L14")
	s.eval(`Array.from(document.querySelectorAll('a')).find(x=>x.textContent==='Jump to step').click()`, nil)
	s.settle()
	s.must(`document.activeElement.id==='step-3' && document.getElementById('step-3').getBoundingClientRect().top >= 50 && document.getElementById('step-3').getBoundingClientRect().top < innerHeight - 100`)
	s.eval(`window.scrollTo(0,0);Array.from(document.querySelectorAll('a')).find(x=>x.textContent==='Jump to step').click()`, nil)
	s.settle()
	s.must(`document.getElementById('step-3').getBoundingClientRect().top < innerHeight - 100`)
	s.eval(`Array.from(document.querySelectorAll('button')).find(x=>x.textContent==='Collapse all').click()`, nil)
	s.must(`Array.from(document.querySelectorAll('.steps details')).every(x=>!x.open)`)
	s.eval(`Array.from(document.querySelectorAll('a')).find(x=>x.textContent.startsWith('View evidence')).click()`, nil)
	s.settle()
	s.must(`document.activeElement.matches('#step-3 .att-h') && document.activeElement.getAttribute('aria-expanded')==='true' && !document.activeElement.nextElementSibling.hidden`)
	s.eval(`document.activeElement.click()`, nil)
	s.must(`document.activeElement.nextElementSibling.hidden && document.activeElement.nextElementSibling.inert`)
	s.key("Tab", 0)
	s.must(`!document.activeElement.closest('.acc[hidden]')`)
	s.eval(`Array.from(document.querySelectorAll('a')).find(x=>x.textContent.startsWith('View evidence')).click()`, nil)
	s.settle()
	s.must(`document.activeElement.matches('#step-3 .att-h') && document.activeElement.getAttribute('aria-expanded')==='true'`)
	s.eval(`Array.from(document.querySelectorAll('button')).find(x=>x.textContent==='Expand all').click()`, nil)
	s.must(`Array.from(document.querySelectorAll('.steps details')).every(x=>x.open) && !document.querySelector('.steps .error-lines').classList.contains('clip')`)
	// Annotation and attachment sidebar controls share the reveal path, including
	// after a filter hides the target. Repeat each action without changing its hash.
	for _, target := range []struct{ selector, focus string }{
		{`.clean-banner a`, `document.activeElement.id==='step-4'`},
		{`.sc-aside a[href*="evidence="]`, `document.activeElement.matches('#step-3 .att-h') && document.activeElement.getAttribute('aria-expanded')==='true'`},
	} {
		for repeat := 0; repeat < 2; repeat++ {
			s.eval(`Array.from(document.querySelectorAll('[role="radio"]')).find(x=>x.textContent==='Problems only')?.click();window.scrollTo(0,0);document.querySelector(`+"`"+target.selector+"`"+`).click()`, nil)
			s.settle()
			s.must(target.focus)
			s.must(`document.activeElement.getBoundingClientRect().top>=0 && document.activeElement.getBoundingClientRect().top<innerHeight && !document.activeElement.closest('[hidden]')`)
		}
	}
}

func TestBrowserRound14EnhancedRetryNavigation(t *testing.T) {
	in := round14Input(t, "retry", "../testdata/captured/cucumber-ruby-9.2.1-retry/retry.json")
	p, e := preparePublicRun([]runInput{in}, renderOptions{Version: 2}, RunConfig{})
	if e != nil {
		t.Fatal(e)
	}
	page, e := renderPublicModel(p.Features, p.Options)
	if e != nil {
		t.Fatal(e)
	}
	s := openPage(t, string(page), true)
	for _, sc := range p.Summary.Scenarios {
		s.goTo("/scenarios/" + sc.ID)
		s.must(`document.querySelector('.v-sc')!==null`)

		if sc.Name == "Always fails" {
			s.must(`Array.from(document.querySelectorAll('.failure-summary a')).some(x=>x.textContent.startsWith('View evidence (15)') && /image|screenshot|png/i.test(x.textContent))`)
			for repeat := 0; repeat < 2; repeat++ {
				s.eval(`Array.from(document.querySelectorAll('button')).find(x=>x.textContent==='Collapse all').click();Array.from(document.querySelectorAll('.failure-summary a')).find(x=>x.textContent.startsWith('View evidence')).click()`, nil)
				s.settle()
				s.must(`document.activeElement.matches('.att-h') && /image\/png/.test(document.activeElement.textContent) && document.activeElement.getAttribute('aria-expanded')==='true' && !document.activeElement.closest('[hidden]')`)
			}
		}
		s.eval(`Array.from(document.querySelectorAll('button')).find(x=>x.textContent==='Collapse all')?.click()`, nil)
		s.must(`Array.from(document.querySelectorAll('.v-sc details')).every(x=>!x.open) && Array.from(document.querySelectorAll('.v-sc .att-h')).every(x=>x.getAttribute('aria-expanded')==='false')`)
		var ax struct {
			Nodes []struct {
				Ignored bool
				Name    struct{ Value string }
			}
		}
		if err := s.b.Call(s.ctx, "Accessibility.getFullAXTree", map[string]any{}, &ax); err != nil {
			t.Fatal(err)
		}
		for _, n := range ax.Nodes {
			if !n.Ignored && (n.Name.Value == "Find in attachment" || n.Name.Value == "Find in error") {
				t.Fatal("collapsed controls remain accessible")
			}
		}
		s.eval(`Array.from(document.querySelectorAll('button')).find(x=>x.textContent==='Expand all')?.click()`, nil)
		s.must(`Array.from(document.querySelectorAll('.v-sc details')).every(x=>x.open) && Array.from(document.querySelectorAll('.v-sc .att-h')).every(x=>x.getAttribute('aria-expanded')==='true')`)
	}
}

func TestBrowserVirtualListReturnPosition(t *testing.T) {
	s := openSession(t, syntheticRun(1, 5000, 1), renderOptions{Version: OptionsVersion}, true)
	s.goTo("/scenarios")
	s.eval(`window.scrollTo(0, 1800)`, nil)
	s.settle()
	s.eval(`(() => {
  const row = [...document.querySelectorAll('.rows .row')].find(x => { const r=x.getBoundingClientRect(); return r.top>250 && r.bottom<innerHeight-80; });
  if (!row) throw new Error('No visible virtual row');
  window.returnAnchor = {id:row.dataset.id, top:row.getBoundingClientRect().top};
  row.focus({preventScroll:true}); row.click();
 })()`, nil)
	s.settle()
	s.must(`document.querySelector('.v-sc') !== null`)
	s.eval(`Array.from(document.querySelectorAll('main a')).find(x => x.textContent.startsWith('Back to ')).click()`, nil)
	s.settle()
	check := `(() => { const row=document.querySelector('.rows .row[data-id="'+window.returnAnchor.id+'"]'); return row && Math.abs(row.getBoundingClientRect().top-window.returnAnchor.top)<2 && document.activeElement===row; })()`
	s.must(check)
	// Browser Back must use the same restoration, without a link's click handler.
	s.eval(`document.querySelector('.rows .row[data-id="'+window.returnAnchor.id+'"]').click()`, nil)
	s.settle()
	s.eval(`history.back()`, nil)
	s.settle()
	s.must(check)
}
