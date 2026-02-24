import { Link, Route, Routes } from "react-router-dom";
import { AppShell } from "./AppShell";
import { HomePage } from "../features/home/HomePage";
import { MetadataPage } from "../features/metadata/MetadataPage";

export function AppRoutes() {
  return (
    <Routes>
      <Route element={<AppShell />}>
        <Route index element={<HomePage />} />
        <Route path="metadata" element={<MetadataPage />} />
        <Route path="*" element={<NotFoundPage />} />
      </Route>
    </Routes>
  );
}

function NotFoundPage() {
  return (
    <section className="panel">
      <h2>Page not found</h2>
      <p className="panel-copy">No route is registered for this path.</p>
      <Link className="panel-action" to="/">
        Back to dashboard
      </Link>
    </section>
  );
}
