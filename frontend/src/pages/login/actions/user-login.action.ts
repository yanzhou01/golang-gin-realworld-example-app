import { redirect } from 'react-router';
import type { ActionFunctionArgs, RouterContextProvider } from 'react-router';
import { setToken } from '~shared/api/auth-storage';
import {
  getGetCurrentUserQueryKey,
  login,
} from '~shared/api/generated/fetch/user-and-authentication/user-and-authentication';
import { LoginBody } from '~shared/api/generated/schemas/loginBody.zod';
import { handleApiError, isApiTransportError } from '~shared/api/handleApiError';
import { queryClient } from '~shared/api/queryClient';
import { validateSchema } from '~shared/api/validateSchema';

export async function userLoginAction({ request }: ActionFunctionArgs<RouterContextProvider>) {
  const formData = await request.formData();
  const validation = validateSchema(LoginBody, { user: Object.fromEntries(formData) });

  if (!validation.ok) {
    return validation;
  }

  try {
    const response = await login(validation.data, { signal: request.signal });
    setToken(response.data.user.token);
    queryClient.setQueryData(getGetCurrentUserQueryKey(), response);
    return redirect('/settings');
  } catch (error) {
    // 401 during login means wrong credentials — show inline form error
    if (isApiTransportError(error) && error.status === 401) {
      return {
        ok: false as const,
        errors: { 'email or password': ['is invalid'] },
      };
    }
    return handleApiError(error);
  }
}

export type UserLoginActionData = typeof userLoginAction;
