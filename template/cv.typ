// Renders one Tailored CV from a single assembled JSON data file.
// The Generation skill writes that JSON (selected + rewritten content,
// already merged with the static parts of data/profile.yaml) and invokes:
//
//   typst compile --root . template/cv.typ output/<slug>/cv.pdf \
//     --input data=output/<slug>/data.json
//
// Kept deliberately dumb: no selection/relevance logic here, only layout.
// Layout (issue #232, ADR-0048): header, About me, Experience grouped by
// employer with a tag row per engagement, Projects, Education (thesis and
// its publication under the degree), one closing line for the remaining
// Static Sections. Every field the layout adds is optional.

// Typst resolves plain relative paths against this file's directory, not
// the project root, so force a root-relative path (requires --root .).
#let data = json("/" + sys.inputs.data)

#set document(title: data.name + " - CV", author: data.name)
#set page(paper: "a4", margin: (x: 1.3cm, y: 1.0cm))
// data.lang is always present in the assembled JSON (no omitempty on the Go
// side), so the "en" default below only ever covers a genuinely empty
// string, not an absent key.
#let lang = data.at("lang", default: "en")
// No ligatures: pdftotext extracts "fl"/"fi" as single glyphs, and an ATS
// then misses "Snowflake". Straight quotes: the clean-user-facing-text step
// (issue #229) keeps curly ones out of the prose.
#set smartquote(enabled: false)
#set text(font: "Liberation Sans", size: 9pt, ligatures: false, fill: rgb("#2E2E2E"), lang: if lang == "" { "en" } else { lang })
#set par(justify: false, leading: 0.48em, spacing: 0.55em)
#set list(marker: text(fill: rgb("#063970"))[•], indent: 0.2em, body-indent: 0.5em, spacing: 0.42em)
#show link: set text(fill: rgb("#063970"))

#let navy = rgb("#063970")
#let grey = rgb("#666666")

#let month-names = ("01": "Jan", "02": "Feb", "03": "Mar", "04": "Apr", "05": "May", "06": "Jun", "07": "Jul", "08": "Aug", "09": "Sep", "10": "Oct", "11": "Nov", "12": "Dec")

// Dates in Master Data are "YYYY" or "YYYY-MM"; render the latter as "Mon YYYY".
// The timeline check (backend/internal/generation/timeline.go) matches
// exactly this wording, "Present" included.
#let fmt-date(s) = {
  if s == none or s == "" { return "Present" }
  let parts = s.split("-")
  if parts.len() == 2 { month-names.at(parts.at(1)) + " " + parts.at(0) } else { s }
}
#let fmt-span(start, end) = if start == end { fmt-date(start) } else { fmt-date(start) + " - " + fmt-date(end) }

// A field or section, tolerating an absent key, an explicit JSON null (a Go
// nil slice marshals to null, and `none.len()` is a hard error) and "".
#let items-of(key) = {
  let v = data.at(key, default: ())
  if v == none { () } else { v }
}
#let field(obj, key) = {
  let v = obj.at(key, default: none)
  if v == none or v == "" { none } else { v }
}

#let section(title) = {
  v(0.4em)
  text(size: 12pt, weight: "bold", fill: navy, upper(title))
  v(-0.25em)
  line(length: 100%, stroke: 1.6pt + navy)
  v(0.1em)
}

// Two rows: title and dates, then the organisation and the place.
#let entry(title, org, dates, place) = grid(
  columns: (1fr, auto),
  row-gutter: 0.35em,
  text(size: 10.5pt, weight: "bold")[#title],
  text(fill: grey)[#dates],
  if org == none [] else { text(weight: "bold", fill: navy)[#org] },
  if place == none [] else { text(fill: grey)[#place] },
)

// A project line: bold name, grey context, and an optional right-aligned span.
#let project(title, sub, span: none) = {
  v(0.55em)
  grid(
    columns: (1fr, auto),
    [#text(weight: "bold")[#title]#if sub != none { text(fill: grey)[ · #sub] }],
    if span == none [] else { text(fill: grey)[#span] },
  )
  v(-0.05em)
}

// Tags as real text in a thin box: an ATS reads them as keywords (no icons).
#let tags(names) = {
  if names == none or names.len() == 0 { return }
  v(-0.1em)
  names.map(n => box(stroke: 0.5pt + rgb("#b0b8c4"), radius: 2.5pt, inset: (x: 3.5pt, y: 2pt), outset: (y: 0.5pt), text(size: 7.8pt, fill: navy)[#n])).join(h(3pt))
  v(-0.15em)
}

#let bullets(items) = if items != none and items.len() > 0 { list(..items) }

// ---- Header ----
// Each contact item is boxed so it can't wrap mid-URL (Typst treats "/" as
// a valid break point otherwise), and email/LinkedIn/GitHub are real links.
#text(size: 22pt, weight: "bold", tracking: 0.5pt)[#upper(data.name)]
#v(-0.5em)
#{
  let where = data.location
  if field(data, "location_note") != none { where = where + " · " + data.location_note }
  grid(columns: (1fr, auto), align: (left + bottom, right + bottom),
    if field(data, "headline") != none { text(size: 13pt, weight: "bold", fill: navy)[#data.headline] } else [],
    text(size: 9.5pt, weight: "bold")[#where],
  )
}
#v(-0.1em)
#text(size: 8.5pt)[
  #box(link("mailto:" + data.email)[#data.email]) #h(0.5em)·#h(0.5em)
  #box[#data.phone] #h(0.5em)·#h(0.5em)
  #box(link("https://linkedin.com/in/" + data.linkedin)[linkedin.com/in/#data.linkedin]) #h(0.5em)·#h(0.5em)
  #box(link("https://github.com/" + data.github)[github.com/#data.github])
]

// ---- About me ----
#if field(data, "summary") != none {
  section("About me")
  data.summary
}

// ---- Experience ----
// Two shapes, per ADR-0045. An In-house Role is one job: role, employer and
// its own span. Client Engagements group under their employer: the block
// states the Employer Tenure (derived from all of that employer's Master
// Data) and, when profile.yaml's employers gives them, the display name and
// the role for the whole employment; each engagement below is a project
// line with no dates of its own (ADR-0048).
//
// Grouping is keyed on employer *and* tenure start, so two separate stints
// at one employer stay two blocks instead of merging into one span that
// would claim the gap between them as employment.
#let exp-kind(exp) = {
  let k = exp.at("kind", default: "")
  if k == "consultancy" or k == "in-house" {
    k
  } else if field(exp, "client") != none {
    "consultancy"
  } else {
    "in-house"
  }
}

// An engagement's client as the CV names it.
#let who-of(exp) = if field(exp, "client_label") != none { exp.client_label } else { field(exp, "client") }

#section("Experience")
#{
  let last-key = none
  let skip = 0
  let all = data.experience
  for (i, exp) in all.enumerate() {
    if skip > 0 {
      skip -= 1
      continue
    }
    let emp-start = exp.at("employer_start", default: exp.start)
    let emp-end = exp.at("employer_end", default: exp.end)
    let employer = if field(exp, "employer_display") != none { exp.employer_display } else { exp.employer }
    let place = field(exp, "location")
    if exp-kind(exp) == "in-house" {
      // Never grouped: each in-house Entry is a job in its own right.
      last-key = none
      v(0.3em)
      entry(exp.role, employer, fmt-span(exp.start, exp.end), place)
      if field(exp, "title") != none { project(exp.title, none) }
    } else {
      let key = exp.employer + "|" + emp-start
      let block-role = field(exp, "employer_role")
      if key != last-key {
        v(0.3em)
        if block-role != none {
          entry(block-role, employer, fmt-span(emp-start, emp-end), place)
        } else {
          entry(employer, none, fmt-span(emp-start, emp-end), place)
        }
        last-key = key
      }
      // Engagements sharing a `group` print as one project: the group is
      // its title, and their clients, tags and bullets are merged. The
      // backend keeps them adjacent (keepGroupsTogether).
      let members = (exp,)
      if field(exp, "group") != none {
        for other in all.slice(i + 1) {
          let same-block = other.employer + "|" + other.at("employer_start", default: other.start) == key
          if same-block and other.at("group", default: none) == exp.group { members.push(other) } else { break }
        }
      }
      if members.len() > 1 {
        skip = members.len() - 1
        let whos = ()
        let merged-tags = ()
        let merged-bullets = ()
        for m in members {
          let w = who-of(m)
          if w != none and w not in whos { whos.push(w) }
          for t in m.at("tags", default: ()) {
            if t not in merged-tags and merged-tags.len() < 10 { merged-tags.push(t) }
          }
          merged-bullets += m.bullets
        }
        project(exp.group, if whos.len() > 0 { whos.join(" · ") } else { none })
        tags(merged-tags)
        bullets(merged-bullets)
        continue
      }
      let who = who-of(exp)
      if field(exp, "title") != none {
        // A titled engagement still names its role when the block does not.
        let sub = if block-role == none and who != none { exp.role + " · " + who } else if block-role == none { exp.role } else { who }
        project(exp.title, sub)
      } else {
        project(exp.role, who)
      }
    }
    tags(exp.at("tags", default: ()))
    bullets(exp.bullets)
  }
}

// ---- Projects ----
#if items-of("projects").len() > 0 {
  section("Projects")
  for p in items-of("projects") {
    let sub = if field(p, "repo") != none {
      let url = if p.repo.starts-with("http") { p.repo } else { "https://" + p.repo }
      link(url)[#p.repo]
    } else { none }
    project(p.name, sub, span: fmt-span(p.start, p.end))
    tags(p.at("tags", default: ()))
    bullets(p.bullets)
  }
}

// ---- Education (Static Section) ----
// A publication whose `degree` names a degree prints under it; the others
// keep their own section below.
#let degrees = items-of("education").map(e => e.degree)
#let pub-line(pub) = {
  let title = if field(pub, "link") != none { link(pub.link)[#pub.title] } else { pub.title }
  [Published: *#title*, #pub.venue]
}
#if items-of("education").len() > 0 {
  section("Education")
  for edu in items-of("education") {
    v(0.2em)
    let title = edu.degree + " " + edu.program + if field(edu, "grade") != none { ", " + edu.grade }
    entry(title, edu.institution, fmt-span(edu.start, edu.end), none)
    let lines = ()
    if field(edu, "thesis") != none { lines.push([Thesis: #edu.thesis]) }
    for pub in items-of("publications") {
      if field(pub, "degree") == edu.degree { lines.push(pub-line(pub)) }
    }
    if edu.at("courses", default: none) != none and edu.courses.len() > 0 {
      lines.push([Courses: #edu.courses.join(", ")])
    }
    bullets(lines)
  }
}

#let standalone = items-of("publications").filter(p => field(p, "degree") == none or p.degree not in degrees)
#if standalone.len() > 0 {
  section("Publications")
  for pub in standalone {
    let title = if field(pub, "link") != none { link(pub.link)[#pub.title] } else { pub.title }
    [*#title*]
    linebreak()
    text(fill: grey)[#pub.authors · #pub.venue]
    v(0.1em)
  }
}

// ---- Closing line: the remaining Static Sections, no headers ----
#{
  let parts = ()
  let langs = items-of("languages")
  if langs.len() > 0 {
    parts.push([*Languages:* #langs.map(l => l.name + " " + l.level).join(", ")])
  }
  let certs = items-of("certifications")
  if certs.len() > 0 {
    parts.push([*Certifications:* #certs.map(c => {
      let t = if field(c, "link") != none { link(c.link)[#c.title] } else [#c.title]
      if field(c, "date") != none [#t (#fmt-date(c.date))] else [#t]
    }).join(", ")])
  }
  let acts = items-of("activities")
  if acts.len() > 0 { parts.push([*Activities:* #acts.map(a => a.title).join(", ")]) }
  let awards = items-of("awards")
  if awards.len() > 0 { parts.push([*Awards:* #awards.map(a => a.title).join(", ")]) }
  if parts.len() > 0 {
    v(0.5em)
    text(size: 8.8pt, parts.join([ · ]))
  }
}
