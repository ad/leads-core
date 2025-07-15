/**
 * API Client for Leads Core Admin Panel
 * Handles all API communication with proper JWT authentication
 */

class APIClient {
    constructor() {
        this.baseURL = window.location.origin;
        this.token = null;
    }

    /**
     * Check if user is authenticated
     */
    isAuthenticated() {
        return window.AuthManager ? window.AuthManager.isAuthenticated() : false;
    }

    /**
     * Set authentication token
     */
    setToken(token) {
        this.token = token;
    }

    /**
     * Clear authentication
     */
    clearAuth() {
        this.token = null;
    }

    /**
     * Get authorization headers
     */
    getAuthHeaders() {
        const headers = {
            'Content-Type': 'application/json'
        };

        const authHeader = window.AuthManager ? window.AuthManager.getAuthHeader() : null;
        if (authHeader) {
            headers['Authorization'] = authHeader;
        }

        return headers;
    }

    /**
     * Make HTTP request with proper error handling
     */
    async makeRequest(url, options = {}) {
        const headers = this.getAuthHeaders();
        
        const requestOptions = {
            ...options,
            headers: {
                ...headers,
                ...options.headers
            }
        };

        try {
            const response = await fetch(url, requestOptions);
            
            // Handle authentication errors
            if (response.status === 401) {
                if (window.AuthManager) {
                    window.AuthManager.logout();
                }
                this.clearAuth();
                throw new Error('Authentication failed. Please login again.');
            }

            // Handle other HTTP errors
            if (!response.ok) {
                const errorData = await response.json().catch(() => ({}));
                throw new Error(errorData.error || `HTTP ${response.status}: ${response.statusText}`);
            }

            // Return JSON response
            return await response.json();
        } catch (error) {
            // Re-throw network errors or other issues
            if (error.name === 'TypeError' && error.message.includes('fetch')) {
                throw new Error('Network error. Please check your connection.');
            }
            throw error;
        }
    }

    /**
     * Test authentication with current token
     */
    async testAuth() {
        if (!this.isAuthenticated()) {
            throw new Error('No valid authentication token');
        }

        try {
            await this.getFormsSummary();
            return true;
        } catch (error) {
            throw new Error('Authentication test failed: ' + error.message);
        }
    }

    /**
     * Get forms summary statistics
     */
    async getFormsSummary() {
        const url = `${this.baseURL}/api/v1/forms/summary`;
        const response = await this.makeRequest(url);
        return response.data;
    }

    /**
     * Get forms list with optional pagination and filtering
     */
    async getForms(options = {}) {
        const url = new URL(`${this.baseURL}/api/v1/forms`);
        
        // Add query parameters
        Object.entries(options).forEach(([key, value]) => {
            if (value !== undefined && value !== null && value !== '') {
                url.searchParams.set(key, value);
            }
        });

        return await this.makeRequest(url.toString());
    }

    /**
     * Get single form by ID
     */
    async getForm(formId) {
        const url = `${this.baseURL}/api/v1/forms/${formId}`;
        const response = await this.makeRequest(url);
        return response.data;
    }

    /**
     * Get form statistics
     */
    async getFormStats(formId) {
        const url = `${this.baseURL}/api/v1/forms/${formId}/stats`;
        const response = await this.makeRequest(url);
        return response.data;
    }

    /**
     * Get form submissions with optional pagination
     */
    async getFormSubmissions(formId, options = {}) {
        const url = new URL(`${this.baseURL}/api/v1/forms/${formId}/submissions`);
        
        // Add query parameters
        Object.entries(options).forEach(([key, value]) => {
            if (value !== undefined && value !== null && value !== '') {
                url.searchParams.set(key, value);
            }
        });

        return await this.makeRequest(url.toString());
    }

    /**
     * Create a new form
     */
    async createForm(formData) {
        const url = `${this.baseURL}/api/v1/forms`;
        return await this.makeRequest(url, {
            method: 'POST',
            body: JSON.stringify(formData)
        });
    }

    /**
     * Update an existing form
     */
    async updateForm(formId, formData) {
        const url = `${this.baseURL}/api/v1/forms/${formId}`;
        return await this.makeRequest(url, {
            method: 'PUT',
            body: JSON.stringify(formData)
        });
    }

    /**
     * Delete a form
     */
    async deleteForm(formId) {
        const url = `${this.baseURL}/api/v1/forms/${formId}`;
        return await this.makeRequest(url, {
            method: 'DELETE'
        });
    }

    /**
     * Health check
     */
    async healthCheck() {
        const url = `${this.baseURL}/health`;
        return await this.makeRequest(url);
    }
}

// Create global instance
window.APIClient = new APIClient();

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = APIClient;
}
