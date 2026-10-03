// Inisial untuk avatar: huruf pertama dari dua kata pertama nama ("Budi Santoso" -> "BS"); "?" bila nama kosong.
export function initialsOf(name: string): string {
  const words = name.trim().split(/\s+/).filter(Boolean);
  return (
    words
      .slice(0, 2)
      .map((w) => w[0]!.toUpperCase())
      .join("") || "?"
  );
}
