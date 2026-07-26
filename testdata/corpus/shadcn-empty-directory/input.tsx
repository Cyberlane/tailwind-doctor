
export function EmptyDirectory() {
  return (
    <Card className="py-24">
      <CardContent className="flex flex-col items-center gap-10">
        <Empty className="min-h-96">
          <EmptyHeader>
            <EmptyMedia
              variant="icon"
              className="size-14 rounded-full bg-muted/70 text-muted-foreground"
            >
              <FileTextIcon className="size-5" />
            </EmptyMedia>
            <EmptyTitle className="font-heading text-2xl tracking-normal normal-case">
              A Blank Canvas
            </EmptyTitle>
            <EmptyDescription>
              Your editorial directory is currently empty. Start building your
              publication&apos;s next issue by drafting the first piece.
            </EmptyDescription>
          </EmptyHeader>
          <EmptyContent>
            <Button>
              <PlusIcon data-icon="inline-start" />
              Create first article
            </Button>
          </EmptyContent>
        </Empty>
        <Separator className="max-w-2xl" />
        <div className="grid w-full max-w-2xl grid-cols-1 gap-8 sm:grid-cols-3">
          {STAGES.map((stage) => (
            <div key={stage.id} className="flex flex-col gap-2">
              <Badge>
                <span
                  aria-hidden
                  className={`size-1.5 rounded-full ${stage.dotClassName}`}
                />

                {stage.label}
              </Badge>
              <p className="text-xs leading-relaxed text-muted-foreground">
                {stage.description}
              </p>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  )
}
