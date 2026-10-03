// Template (bukan layout): dirender ulang di setiap perpindahan halaman, sehingga isi halaman selalu masuk dengan
// animasi naik yang halus. Sidebar dan navbar ada di layout dan tidak ikut beranimasi.
export default function DashboardTemplate({ children }: { children: React.ReactNode }) {
  return <div className="page-enter">{children}</div>;
}
