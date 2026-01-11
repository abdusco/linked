function app() {
	return {
		links: [],
		loading: true,
		message: { text: '', type: '' },
		pollInterval: null,

		init() {
			this.loadLinks();
			this.startPolling();
			document.addEventListener('visibilitychange', () => {
				if (document.hidden) {
					this.stopPolling();
				} else {
					this.startPolling();
				}
			});
		},

		startPolling() {
			if (!this.pollInterval) {
				this.pollInterval = setInterval(() => this.loadLinks(), 5000);
			}
		},

		stopPolling() {
			if (this.pollInterval) {
				clearInterval(this.pollInterval);
				this.pollInterval = null;
			}
		},

		async loadLinks() {
			try {
				const response = await fetch('/api/links');
				if (response.ok) {
					const data = await response.json();
					this.links = data.links || [];
				}
			} catch (error) {
				console.error('Error loading links:', error);
			} finally {
				this.loading = false;
			}
		},

		async createLink() {
			const url = document.getElementById('url').value;
			const slug = document.getElementById('slug').value;

			if (!url) {
				this.showMessage('URL is required', 'error');
				return;
			}

			this.loading = true;
			try {
				const response = await fetch('/api/links', {
					method: 'POST',
					headers: {
						'Content-Type': 'application/json'
					},
					body: JSON.stringify({ url, slug: slug || undefined })
				});

				if (response.ok) {
					this.showMessage('Link created successfully!', 'success');
					document.getElementById('url').value = '';
					document.getElementById('slug').value = '';
					await this.loadLinks();
					setTimeout(() => this.message.text = '', 3000);
				} else {
					const data = await response.json();
					this.showMessage(data.message || 'Error creating link', 'error');
				}
			} catch (error) {
				this.showMessage('Error creating link: ' + error.message, 'error');
			} finally {
				this.loading = false;
			}
		},

		showMessage(text, type) {
			this.message = { text, type };
		},

		formatDate(dateString) {
			const date = new Date(dateString);
			return date.toLocaleString();
		}
	};
}

