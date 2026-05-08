// See https://svelte.dev/docs/kit/types#app
declare global {
	namespace App {
		interface Locals {
			user?: {
				id: string;
				username: string;
			};
		}
		// interface Error {}
		// interface PageData {}
		// interface Platform {}
	}
}

export {};
