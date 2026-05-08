import type { Handle } from '@sveltejs/kit';

/**
 * Server hook. Currently a passthrough; when we add admin auth this is where
 * we'll:
 *   - read a session cookie
 *   - verify it (Discord OAuth result stored server-side)
 *   - attach the resolved user to `event.locals.user`
 *
 * Routes under src/routes/(admin) check `locals.user` and 401 otherwise.
 */
export const handle: Handle = async ({ event, resolve }) => {
	// TODO: session/cookie -> event.locals.user (Discord OAuth)
	return resolve(event);
};
