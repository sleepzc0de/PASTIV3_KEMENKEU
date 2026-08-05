import { z } from "zod";

export const loginSchema = z.object({
  username: z
    .string()
    .min(1, "Username atau email wajib diisi"),
  password: z
    .string()
    .min(6, "Password minimal 6 karakter"),
});

export type LoginFormData = z.infer<typeof loginSchema>;