// Renders one Cover Letter from a single assembled JSON data file, the
// same convention as cv.typ. The app's Render step (see
// backend/internal/generation/render.go) writes that JSON and invokes:
//
//   typst compile --root . template/cover-letter.typ output/<slug>/cover-letter.pdf \
//     --input data=output/<slug>/cover-letter-data.json
//
// Kept deliberately dumb: no drafting logic here, only layout.

// Typst resolves plain relative paths against this file's directory, not
// the project root, so force a root-relative path (requires --root .).
#let data = json("/" + sys.inputs.data)

#set page(paper: "a4", margin: (x: 2.2cm, y: 2.2cm))
// Straight quotes: Typst would turn ' and " into curly ones, which the
// clean-user-facing-text step (issue #229) keeps out of the prose.
#set smartquote(enabled: false)
#set text(font: "Liberation Sans", size: 10.5pt, lang: "en")
#set par(justify: false, leading: 0.65em)

#text(size: 9.5pt)[
  #box[#data.name] #h(0.5em)•#h(0.5em)
  #box[#data.location] #h(0.5em)•#h(0.5em)
  #box(link("mailto:" + data.email)[#data.email]) #h(0.5em)•#h(0.5em)
  #box[#data.phone] #h(0.5em)•#h(0.5em)
  #box(link("https://linkedin.com/in/" + data.linkedin)[linkedin.com/in/#data.linkedin]) #h(0.5em)•#h(0.5em)
  #box(link("https://github.com/" + data.github)[github.com/#data.github])
]

#v(1.5em)

#data.body
