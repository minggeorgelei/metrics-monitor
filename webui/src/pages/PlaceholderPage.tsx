/**
 * PlaceholderPage is shown on routes whose dashboards aren't built
 * yet. We render the menu structure now so the navigation experience
 * is complete from day one — content gets filled in as the matching
 * backend inputs land.
 */
interface Props {
  title: string;
  description?: string;
}

export function PlaceholderPage({
  title,
  description = "This dashboard isn't built yet — the supporting input plugin will land in a later phase. The route is reserved so the navigation experience stays consistent.",
}: Props) {
  return (
    <div className="mx-auto my-20 max-w-[560px] text-center">
      <h1 className="mb-3 text-[34px] font-bold leading-[1.08] tracking-normal text-fg-strong max-[760px]:text-[28px]">
        {title}
      </h1>
      <p className="mb-4 text-[15px] font-semibold uppercase tracking-[0.08em] text-muted">
        Coming soon
      </p>
      <p className="text-[15px] leading-[1.55] text-muted">{description}</p>
    </div>
  );
}
