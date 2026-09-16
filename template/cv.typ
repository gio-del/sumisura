// Renders one Tailored CV from a single assembled JSON data file.
// The Generation skill writes that JSON (selected + rewritten content,
// already merged with the static parts of data/profile.yaml) and invokes:
//
//   typst compile --root . template/cv.typ output/<slug>/cv.pdf \
//     --input data=output/<slug>/data.json
//
// Kept deliberately dumb: no selection/relevance logic here, only layout.

// Typst resolves plain relative paths against this file's directory, not
// the project root, so force a root-relative path (requires --root .).
#let data = json("/" + sys.inputs.data)

#set page(paper: "a4", margin: (x: 1.8cm, y: 1.2cm))
// data.lang is always present in the assembled JSON (no omitempty on the Go
// side), so the "en" default below only ever covers a genuinely empty
// string, not an absent key.
#let lang = data.at("lang", default: "en")
#set text(font: "Liberation Sans", size: 9.5pt, lang: if lang == "" { "en" } else { lang })
#set par(justify: false, leading: 0.5em, spacing: 0.5em)
#set list(marker: [•], indent: 0.4em)

#let accent = rgb("#1a4d8f")

#let month-names = ("01": "Jan", "02": "Feb", "03": "Mar", "04": "Apr", "05": "May", "06": "Jun", "07": "Jul", "08": "Aug", "09": "Sep", "10": "Oct", "11": "Nov", "12": "Dec")

// Dates in Master Data are "YYYY" or "YYYY-MM"; render the latter as "Mon YYYY".
#let fmt-date(s) = {
  if s == none { return "present" }
  let parts = s.split("-")
  if parts.len() == 2 { month-names.at(parts.at(1)) + " " + parts.at(0) } else { s }
}

// A section's items, tolerating both an absent key and an explicit JSON
// null (a Go nil slice marshals to null, and `none.len()` is a hard error).
#let items-of(key) = {
  let v = data.at(key, default: ())
  if v == none { () } else { v }
}

#let section(title) = [
  #v(0.3em)
  #text(weight: "bold", size: 11pt, fill: accent)[#title]
  #v(-0.4em)
  #line(length: 100%, stroke: 0.6pt + accent)
  #v(0.05em)
]

#let dated-row(left, start, end) = grid(
  columns: (1fr, auto),
  left,
  text(size: 9pt)[#if start == end [#fmt-date(start)] else [#fmt-date(start) -- #fmt-date(end)]],
)

// ---- Header ----
// Each contact item is boxed so it can't wrap mid-URL (Typst treats "/" as
// a valid break point otherwise), and email/LinkedIn/GitHub are real links.
#align(center)[
  #text(size: 20pt, weight: "bold")[#data.name]
  #v(0.08em)
  #text(size: 9pt)[
    #box[#data.location] #h(0.5em)•#h(0.5em)
    #box(link("mailto:" + data.email)[#data.email]) #h(0.5em)•#h(0.5em)
    #box[#data.phone] #h(0.5em)•#h(0.5em)
    #box(link("https://linkedin.com/in/" + data.linkedin)[linkedin.com/in/#data.linkedin]) #h(0.5em)•#h(0.5em)
    #box(link("https://github.com/" + data.github)[github.com/#data.github])
  ]
]

// ---- Education ----
#section("Education")
#for edu in data.education [
  #dated-row([*#edu.degree* -- #edu.institution, #edu.program#if "grade" in edu [ (#edu.grade)]], edu.start, edu.end)
  #if "courses" in edu and edu.courses.len() > 0 [
    #text(size: 8.5pt)[Courses: #edu.courses.join(", ")]
  ]
  #v(0.12em)
]

// ---- Experience (grouped by employer; each client engagement is its own sub-block) ----
#section("Experience")
#{
  let last-employer = none
  for exp in data.experience {
    if exp.employer != last-employer {
      dated-row([*#exp.role* -- #exp.employer#if "location" in exp [, #exp.location]], exp.start, exp.end)
      last-employer = exp.employer
    }
    if "client" in exp and exp.client != none {
      text(style: "italic", size: 9pt)[#exp.client]
      linebreak()
    }
    list(..exp.bullets)
    v(0.12em)
  }
}

// ---- Projects ----
#if "projects" in data and data.projects.len() > 0 [
  #section("Projects")
  #for p in data.projects [
    #dated-row([*#p.name*#if "repo" in p [ -- #p.repo]], p.start, p.end)
    #list(..p.bullets)
    #v(0.12em)
  ]
]

// ---- Tech Stack (derived by the skill from selected entries' tags) ----
#if items-of("tech_stack").len() > 0 [
  #section("Tech Stack")
  #items-of("tech_stack").join(", ")
]

// ---- Publications (Static Section) ----
#if items-of("publications").len() > 0 [
  #section("Publications")
  #for pub in items-of("publications") [
    #if "link" in pub [
      *#link(pub.link)[#pub.title]*
    ] else [
      *#pub.title*
    ]
    #linebreak()
    #text(size: 8.5pt)[#pub.authors --- #pub.venue]
    #v(0.1em)
  ]
]

// ---- Certifications (Static Section) ----
#if items-of("certifications").len() > 0 [
  #section("Certifications")
  #for c in items-of("certifications") [
    #let label = if "link" in c and c.link != "" { link(c.link)[#c.title] } else { c.title }
    *#label*#if "issuer" in c and c.issuer != "" [ -- #c.issuer]#if "date" in c and c.date != "" [ (#fmt-date(c.date))]
    #linebreak()
  ]
]

// ---- Awards (Static Section) ----
#if items-of("awards").len() > 0 [
  #section("Awards")
  #for a in items-of("awards") [
    *#a.title*: #a.description
  ]
]

// ---- Activities (Static Section) ----
#if items-of("activities").len() > 0 [
  #section("Activities")
  #for a in items-of("activities") [
    *#a.title*: #a.description
  ]
]

// ---- Languages (Static Section) ----
#if items-of("languages").len() > 0 [
  #section("Languages")
  #items-of("languages").map(l => [*#l.name*: #l.level]).join([ #h(0.5em) • #h(0.5em) ])
]
