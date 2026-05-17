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
    <div className="placeholder">
      <h1>{title}</h1>
      <p className="placeholder__lede">Coming soon</p>
      <p className="placeholder__body">{description}</p>
    </div>
  );
}
