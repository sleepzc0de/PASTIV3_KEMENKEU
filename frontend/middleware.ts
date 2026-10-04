import { NextResponse } from "next/server";
import type { NextRequest } from "next/server";
import { COOKIE_TOKEN, jalurMasukDari, samakanJalur, tentukanAksi } from "@/lib/loginPath";

// Alamat halaman login: LOGIN_PATH disisipkan saat build lewat next.config.ts (bawaan: frontend/login-path.txt); /login biasa
// dijawab 404. Aturan lengkapnya ada di lib/loginPath.ts.
const JALUR_MASUK = jalurMasukDari(process.env.LOGIN_PATH);

// Rute yang tidak ada: jawabannya halaman 404 bawaan aplikasi.
const RUTE_TIDAK_ADA = "/halaman-tidak-ada";

export function middleware(request: NextRequest) {
  const aksi = tentukanAksi({
    pathname: samakanJalur(request.nextUrl.pathname, JALUR_MASUK),
    search: request.nextUrl.search,
    adaToken: Boolean(request.cookies.get(COOKIE_TOKEN)?.value),
    jalurMasuk: JALUR_MASUK,
  });

  // Alamat tujuan (path dan query) pada host yang sama dengan permintaan; middleware Next menolak Location relatif.
  const tujuan = (ke: string) => {
    const url = request.nextUrl.clone();
    const [pathname, search = ""] = ke.split("?");
    url.pathname = pathname;
    url.search = search ? "?" + search : "";
    return url;
  };

  switch (aksi.jenis) {
    case "lanjut":
      return NextResponse.next();

    case "alihkan": {
      const res = NextResponse.redirect(tujuan(aksi.ke));
      res.headers.set("Cache-Control", "no-store");
      return res;
    }

    case "tulis-ulang": {
      const res = NextResponse.rewrite(tujuan(aksi.ke));
      res.headers.set("Cache-Control", "no-store");
      return res;
    }

    case "tidak-ada": {
      const url = request.nextUrl.clone();
      url.pathname = RUTE_TIDAK_ADA;
      url.search = "";
      const res = NextResponse.rewrite(url, { status: 404 });
      res.headers.set("Cache-Control", "no-store");
      return res;
    }
  }
}

// Semua halaman kecuali berkas statis (yang memuat titik, mis. .js .css .png .ico) dan internal Next.
export const config = {
  matcher: ["/((?!_next/|.*\\.[a-zA-Z0-9]+$).*)"],
};
