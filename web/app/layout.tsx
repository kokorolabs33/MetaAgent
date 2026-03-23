import type { Metadata } from "next";
import { Inter } from "next/font/google";
import "./globals.css";
import { Nav } from "@/components/ui/nav";
import { OrgProvider } from "@/components/OrgProvider";

const inter = Inter({ subsets: ["latin"] });

export const metadata: Metadata = {
  title: "TaskHub",
  description: "Enterprise multi-agent collaboration platform",
};

export default function RootLayout({
  children,
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en" className="dark">
      <body className={`${inter.className} bg-gray-950 text-gray-100 min-h-screen`}>
        <div className="flex h-screen overflow-hidden">
          <Nav />
          <main className="flex-1 overflow-auto">
            <OrgProvider>{children}</OrgProvider>
          </main>
        </div>
      </body>
    </html>
  );
}
