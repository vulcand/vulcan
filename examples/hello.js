function handle(request) {
    var forward = {
        rates: {
            "token": ["10 requests/minute", "100 KB/minute"]
        },
        upstreams: ["http://localhost:5000"]
    };
    return forward;
}

function handleError(request, error) {
    return error
}
