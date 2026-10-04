# Reading document formats (PDF, DOCX)

The `read` tool extracts plain text directly from `.pdf` and `.docx`
files — pass the file path exactly like you would for a `.txt`/`.go`/
`.md` file, no special handling needed on your end. You don't need to
ask the user to convert the file first.

**What you get back:** plain text content, in reading order for DOCX
(paragraph by paragraph) and in the PDF's internal text order (which can
occasionally differ from visual order for complex multi-column layouts).

**What's lost in extraction:** formatting (bold/italic/font), tables
(DOCX tables aren't extracted — only paragraph body text), images, and
headers/footers. If the user needs table data specifically from a DOCX
and the extracted text looks wrong/missing for that part, say so plainly
rather than guessing at numbers that aren't actually in the extracted
text.

**Scanned PDFs** (a PDF that's really just images of pages, common for
scanned paper documents) have no extractable text layer — the read
tool will report an error like "no extractable text" rather than
silently returning nothing. There's no OCR capability available — tell
the user this directly rather than fabricating what the document might
say.

**Writing these formats back out is NOT supported** — `write` only
produces plain text/code files. If asked to "save this as a PDF" or
"create a Word document," say that directly rather than writing a
`.pdf`/`.docx` extension on a plain-text file (which would just create a
broken/unopenable file, not a real document in that format). Markdown
(`.md`) is always a safe, real alternative to offer instead.
