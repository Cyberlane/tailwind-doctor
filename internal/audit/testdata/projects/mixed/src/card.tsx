export const Card = ({ active }: { active: boolean }) => (
  <div className="rounded-lg border-r border-gray-200 shadow-[0_1px_2px_rgba(0,0,0,0.1)]">
    <span className={active ? "font-bold" : "font-normal"} />
  </div>
);
