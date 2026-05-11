import { NextResponse, type NextRequest } from "next/server";

const DEFAULT_WORKSPACE_SLUG = "camus-workspace-pilot";
const DEFAULT_EMAIL = "camus-local@multica.local";
const LOCAL_AUTOLOGIN_COOKIE = "multica_local_autologin";

function localAutologinEnabled(): boolean {
  return process.env.MULTICA_LOCAL_AUTOLOGIN_ENABLED === "true";
}

function autologinApiBase(): string {
  return (
    process.env.MULTICA_LOCAL_AUTOLOGIN_API_URL ||
    process.env.NEXT_PUBLIC_API_URL ||
    "http://localhost:8080"
  ).replace(/\/+$/, "");
}

function splitSetCookieHeader(value: string | null): string[] {
  if (!value) return [];
  return value.split(/,(?=\s*[^;,]+=)/g).map((part) => part.trim());
}

function getSetCookieHeaders(headers: Headers): string[] {
  const withGetSetCookie = headers as Headers & {
    getSetCookie?: () => string[];
  };
  return withGetSetCookie.getSetCookie?.() ?? splitSetCookieHeader(headers.get("set-cookie"));
}

async function postJson(path: string, payload: unknown): Promise<Response> {
  return fetch(`${autologinApiBase()}${path}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
    cache: "no-store",
  });
}

function publicUrl(req: NextRequest, pathname: string): URL {
  const proto = req.headers.get("x-forwarded-proto") || "http";
  const host = req.headers.get("x-forwarded-host") || req.headers.get("host");
  if (!host) return new URL(pathname, req.url);
  return new URL(pathname, `${proto}://${host}`);
}

export async function GET(req: NextRequest) {
  if (!localAutologinEnabled()) {
    return NextResponse.json({ error: "local autologin is disabled" }, { status: 404 });
  }

  const email = process.env.MULTICA_LOCAL_AUTOLOGIN_EMAIL || DEFAULT_EMAIL;
  const code = process.env.MULTICA_LOCAL_AUTOLOGIN_CODE;
  const workspaceSlug =
    process.env.MULTICA_LOCAL_AUTOLOGIN_WORKSPACE_SLUG || DEFAULT_WORKSPACE_SLUG;

  if (!code) {
    return NextResponse.json(
      { error: "MULTICA_LOCAL_AUTOLOGIN_CODE is required" },
      { status: 500 },
    );
  }

  const sendRes = await postJson("/auth/send-code", { email });
  if (!sendRes.ok) {
    return NextResponse.json(
      { error: "failed to request local verification code" },
      { status: 502 },
    );
  }

  const verifyRes = await postJson("/auth/verify-code", { email, code });
  if (!verifyRes.ok) {
    return NextResponse.json(
      { error: "failed to verify local autologin code" },
      { status: 502 },
    );
  }

  const redirectTo = publicUrl(req, `/${workspaceSlug}/issues`);
  const res = NextResponse.redirect(redirectTo);
  for (const cookie of getSetCookieHeaders(verifyRes.headers)) {
    res.headers.append("Set-Cookie", cookie);
  }
  res.headers.append(
    "Set-Cookie",
    "multica_logged_in=1; Path=/; Max-Age=31536000; SameSite=Lax",
  );
  res.headers.append(
    "Set-Cookie",
    `last_workspace_slug=${encodeURIComponent(workspaceSlug)}; Path=/; Max-Age=31536000; SameSite=Lax`,
  );
  res.headers.append(
    "Set-Cookie",
    `${LOCAL_AUTOLOGIN_COOKIE}=1; Path=/; Max-Age=31536000; SameSite=Lax`,
  );
  return res;
}
