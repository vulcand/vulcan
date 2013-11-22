function handle(request) {
    var forward = {
        rates: {
        },
        upstreams: ["http://localhost:5000/fqfwafwaf"]
    };
    return forward;
}

function handleError(request, error) {
    return error
}
