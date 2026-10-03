import { notFound } from "next/navigation";

// "/" dijawab oleh middleware (arahkan ke /dashboard bila ada sesi, ke halaman login bagi peramban yang mengenalnya, selain itu
// 404). Halaman ini hanya tercapai bila middleware tidak berjalan, dan tidak boleh membocorkan alamat login.
export default function RootPage() {
  notFound();
}
