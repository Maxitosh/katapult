// @cpt-dod:cpt-katapult-dod-web-ui-transfer-dashboard:p1
// @cpt-dod:cpt-katapult-dod-web-ui-agent-browser:p2
// @cpt-dod:cpt-katapult-dod-web-ui-documentation:p2

import { NavLink } from "react-router-dom"
import { cn } from "@/lib/utils"
import { Separator } from "@/components/ui/separator"

interface SidebarProps {
  onNavigate?: () => void
}

const navItems = [
  {
    to: "/",
    label: "Transfers",
    icon: (
      <svg
        width="16"
        height="16"
        viewBox="0 0 16 16"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <line x1="2" y1="4" x2="14" y2="4" />
        <line x1="2" y1="8" x2="14" y2="8" />
        <line x1="2" y1="12" x2="14" y2="12" />
      </svg>
    ),
    end: true,
  },
  {
    to: "/transfers/new",
    label: "New Transfer",
    icon: (
      <svg
        width="16"
        height="16"
        viewBox="0 0 16 16"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <line x1="8" y1="3" x2="8" y2="13" />
        <line x1="3" y1="8" x2="13" y2="8" />
      </svg>
    ),
    end: false,
  },
  {
    to: "/agents",
    label: "Agents",
    icon: (
      <svg
        width="16"
        height="16"
        viewBox="0 0 16 16"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
      >
        <rect x="3" y="2" width="10" height="12" rx="1" />
        <line x1="6" y1="5" x2="10" y2="5" />
        <line x1="6" y1="8" x2="10" y2="8" />
        <line x1="6" y1="11" x2="10" y2="11" />
      </svg>
    ),
    end: false,
  },
]

const helpLinks = [
  { label: "API Documentation", href: "#" },
  { label: "Onboarding Guide", href: "#" },
]

export function Sidebar({ onNavigate }: SidebarProps) {
  return (
    <div className="flex h-full flex-col">
      {/* App title */}
      <div className="flex h-12 items-center border-b px-4">
        <span className="text-lg font-bold tracking-tight">Katapult</span>
      </div>

      {/* Navigation */}
      <nav className="flex-1 space-y-1 p-3">
        {navItems.map((item) => (
          <NavLink
            key={item.to}
            to={item.to}
            end={item.end}
            onClick={onNavigate}
            className={({ isActive }) =>
              cn(
                "flex items-center gap-2.5 rounded-md px-3 py-2 text-sm font-medium transition-colors",
                isActive
                  ? "bg-primary/10 text-primary"
                  : "text-muted-foreground hover:bg-muted hover:text-foreground"
              )
            }
          >
            {item.icon}
            {item.label}
          </NavLink>
        ))}
      </nav>

      {/* Help section */}
      <div className="p-3">
        <Separator className="mb-3" />
        <p className="mb-2 px-3 text-xs font-medium uppercase tracking-wider text-muted-foreground">
          Help
        </p>
        {helpLinks.map((link) => (
          <a
            key={link.label}
            href={link.href}
            className="block rounded-md px-3 py-1.5 text-sm text-muted-foreground transition-colors hover:text-foreground"
          >
            {link.label}
          </a>
        ))}
      </div>
    </div>
  )
}
