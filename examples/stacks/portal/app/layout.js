export const metadata = {
  title: "portal",
  description: "litefaas Next.js stack example",
};

export default function RootLayout({ children }) {
  return (
    <html lang="en">
      <body>{children}</body>
    </html>
  );
}
