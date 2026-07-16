export interface LoginResponse {
  token: string;
  expiresIn: number;
}

export interface Session {
  token: string;
  user: string;
  roles: string[];
}
