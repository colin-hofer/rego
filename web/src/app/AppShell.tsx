import { NavLink, Outlet } from "react-router-dom";

export function AppShell() {
  return (
    <main className="page">
      <div className="glow glow-a" />
      <div className="glow glow-b" />

      <section className="hero">
        <p className="eyebrow">Go-Centric React Starter</p>
        <h1>Rego starter app</h1>
        <p className="lede">
          Add backend features in <code>internal/modules</code> and frontend features in <code>web/src/features</code>.
        </p>

        <nav className="route-nav" aria-label="app navigation">
          <NavLink end to="/" className={({ isActive }) => (isActive ? "route-link route-link-active" : "route-link")}>
            Dashboard
          </NavLink>
          <NavLink
            to="/metadata"
            className={({ isActive }) => (isActive ? "route-link route-link-active" : "route-link")}
          >
            Metadata
          </NavLink>
        </nav>

        <section className="app-content">
          <Outlet />
        </section>
      </section>
    </main>
  );
}
