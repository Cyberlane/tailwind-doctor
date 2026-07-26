// Authored for the extraction corpus. Every class-list-shaped string below is a
// decoy: none of them is a class attribute, so none may be extracted.

// className = "p-4 m-2 text-sm" is mentioned here in a comment, not written.
/* A block comment showing the old API: class="flex items-center gap-2" */

const documentationSnippet = 'class="grid grid-cols-3 gap-4"'

const helpText = `Set className="rounded-lg border" on the wrapper element.`

class ButtonRegistry {
  private readonly entries = new Map<string, string>()
}

export function Decoys() {
  return (
    <section className="flex flex-col gap-4">
      <p aria-label="write class = 'p-8 shadow' to style it">{documentationSnippet}</p>
      <code data-example='class="underline decoration-dotted"'>{helpText}</code>
      <ButtonRegistryView registry={new ButtonRegistry()} />
    </section>
  )
}
