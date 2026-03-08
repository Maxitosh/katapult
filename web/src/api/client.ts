// @cpt-dod:cpt-katapult-dod-web-ui-transfer-dashboard:p1

const BASE_URL = "/api/v1alpha1";

function getAuthToken(): string {
  return import.meta.env.VITE_API_TOKEN ?? "";
}

export class ApiError extends Error {
  status: number;
  override message: string;

  constructor(status: number, message: string) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.message = message;
  }
}

async function handleResponse<T>(response: Response): Promise<T> {
  if (!response.ok) {
    let message = response.statusText;
    try {
      const body = await response.json();
      message = body.error ?? body.message ?? message;
    } catch {
      // use default statusText
    }
    throw new ApiError(response.status, message);
  }
  return response.json() as Promise<T>;
}

function authHeaders(): HeadersInit {
  return {
    Authorization: `Bearer ${getAuthToken()}`,
    "Content-Type": "application/json",
  };
}

export async function apiGet<T>(path: string): Promise<T> {
  const response = await fetch(`${BASE_URL}${path}`, {
    method: "GET",
    headers: authHeaders(),
  });
  return handleResponse<T>(response);
}

export async function apiPost<T>(path: string, body: unknown): Promise<T> {
  const response = await fetch(`${BASE_URL}${path}`, {
    method: "POST",
    headers: authHeaders(),
    body: JSON.stringify(body),
  });
  return handleResponse<T>(response);
}

export async function apiDelete<T>(path: string): Promise<T> {
  const response = await fetch(`${BASE_URL}${path}`, {
    method: "DELETE",
    headers: authHeaders(),
  });
  return handleResponse<T>(response);
}
