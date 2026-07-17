export interface Style {
  name: string;
  format: string;
  workspace?: string;
}

export interface StyleContent {
  name: string;
  format: string;
  content: string;
}

export interface ValidationError {
  line: number;
  message: string;
}

export interface ValidationResult {
  ok: boolean;
  errors: ValidationError[] | null;
}
