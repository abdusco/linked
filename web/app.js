class UnauthenticatedError extends Error {
	constructor(message) {
		super(message);
		this.name = 'UnauthenticatedError';
	}
}

function loginApp() {
	return {
		username: '',
		password: '',
		submitting: false,
		errorMessage: '',

		async login() {
			this.errorMessage = '';
			this.submitting = true;

			try {
				await fetchJSON('/login', {
					method: 'POST',
					body: { username: this.username, password: this.password }
				});

				window.location.href = '/dashboard';
			} catch (error) {
				this.errorMessage = error.message || 'An error occurred';
			} finally {
				this.submitting = false;
			}
		},
	};
}

function app() {
	return {
		links: [],
		loading: true,
		creating: false,
		message: { text: '', type: '' },
		messageTimeout: null,

		init() {
			this.loadLinks();
		},

		async loadLinks() {
			this.loading = true;
			try {
				const response = await fetchJSON('/api/links');
				this.links = response?.links || [];
			} catch (error) {
				this.handleError(error);
			} finally {
				this.loading = false;
			}
		},

		async createLink() {
			const url = document.getElementById('url').value;
			const slug = document.getElementById('slug').value;

			if (!url) {
				this.showError('URL is required');
				return;
			}

			this.creating = true;
			try {
				const response = await fetchJSON('/api/links', {
					method: 'POST',
					body: { url, slug: slug || undefined }
				});

				if (response) {
					this.showMessage('Link created successfully!', 'success');
					document.getElementById('url').value = '';
					document.getElementById('slug').value = '';
					await this.loadLinks();
				}
			} catch (error) {
				this.handleError(error);
			} finally {
				this.creating = false;
			}
		},

		async deleteLink(id, slug) {
			if (!confirm(`Are you sure you want to delete the link "${slug}"? This action cannot be undone.`)) {
				return;
			}

			this.loading = true;
			try {
				await fetchJSON(`/api/links/${id}`, {
					method: 'DELETE'
				});

				this.showMessage('Link deleted successfully!', 'success');
				await this.loadLinks();
			} catch (error) {
				this.handleError(error);
			} finally {
				this.loading = false;
			}
		},

		showMessage(text, type) {
			this.clearMessageTimeout();
			this.message = { text, type };

			if (type === 'success') {
				this.messageTimeout = setTimeout(() => {
					this.message.text = '';
				}, 5000);
			}
		},

		showError(text) {
			this.showMessage(text, 'error');
		},

		handleError(error) {
			if (error instanceof UnauthenticatedError) {
				window.location.href = '/';
				return;
			}
			this.showError(error.message);
		},

		clearMessageTimeout() {
			if (this.messageTimeout) {
				clearTimeout(this.messageTimeout);
				this.messageTimeout = null;
			}
		},

		formatDate(dateString) {
			const date = new Date(dateString);
			return date.toLocaleString();
		},
	};
}


function copyToClipboard(text) {
	const textarea = document.createElement('textarea');
	textarea.value = text;
	textarea.style.position = 'fixed';
	textarea.style.opacity = '0';

	document.body.appendChild(textarea);
	textarea.select();

	try {
		document.execCommand('copy');
		console.log('Copied!');
	} catch (err) {
		console.error('Failed to copy:', err);
	}

	document.body.removeChild(textarea);
}

/**
 * Fetch JSON from the given URL with the given options.
 * @param {string} url - The URL to fetch from.
 * @param {RequestInit} options - The options to pass to the fetch function.
 * @returns {Promise<Object | null>} - The JSON response or null if the status is 204.
 * @throws {UnauthenticatedError} If status is 401
 * @throws {Error} If status >= 400, throws error with API message
 */
async function fetchJSON(url, options) {
	let { headers, body, ...rest } = options || {};
	const response = await fetch(url, {
		headers: {
			'Content-Type': 'application/json',
			...headers
		},
		body: body ? JSON.stringify(body) : undefined,
		...rest
	});

	if (response.status === 204) {
		return null;
	}

	const data = await response.json();

	if (response.status === 401) {
		const errorMessage = data?.error || data?.message || 'Unauthorized';
		throw new UnauthenticatedError(errorMessage);
	}

	if (response.status >= 400) {
		const errorMessage = data?.error || data?.message || `HTTP error! status: ${response.status}`;
		throw new Error(errorMessage);
	}

	return data;
}