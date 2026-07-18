export interface Style {
  name: string;
  format: string;
  workspace?: string;
}

import type { StyleModel } from "@/lib/sld";

export interface StyleContent {
  name: string;
  format: string;
  content: string;
  model?: StyleModel;
}

export interface ValidationError {
  line: number;
  message: string;
}

export interface ValidationResult {
  ok: boolean;
  errors: ValidationError[] | null;
}
