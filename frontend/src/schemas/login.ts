import { z } from 'zod';

export const LoginFormSchema = z.object({
  username: z.string().min(1, 'username'),
  password: z.string().min(1, 'password'),
  twoFactorCode: z.string().optional(),
});

// The login page cannot know the account role before authentication. Customer
// accounts do not share the administrator's panel-wide TOTP secret, so an empty
// value must be allowed here; the backend still requires a valid code for admin.
export const TwoFactorCodeSchema = z
  .string()
  .refine((value) => value === '' || /^\d{6}$/.test(value), 'pages.settings.security.twoFactorModalError');

export const TotpCodeSchema = z
  .string()
  .regex(/^\d{6}$/, 'pages.settings.security.twoFactorModalError');

export type LoginFormValues = z.infer<typeof LoginFormSchema>;
