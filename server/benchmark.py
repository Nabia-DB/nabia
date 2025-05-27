import requests
import time
import threading
import random
import string

BASE_URL = "http://localhost:5380"  # Replace with your REST API endpoint
NUM_REQUESTS = 10000
NUM_THREADS = 1

def random_string(length=10):
    """Generate a random alphanumeric string."""
    return ''.join(random.choices(string.ascii_letters + string.digits, k=length))

def worker(thread_id, results):
    """Perform PUT and GET requests in a thread."""
    session = requests.Session()
    for _ in range(NUM_REQUESTS // NUM_THREADS):
        key = random_string()
        value = random_string(20)

        try:
            # PUT request (store data)
            put_response = session.put(f"{BASE_URL}/{key}", json={"value": value})
            if put_response.status_code in [200, 201]:
                results["PUT_success"] += 1
            results["PUT_total"] += 1

            # GET request (retrieve data)
            get_response = session.get(f"{BASE_URL}/{key}")
            if get_response.status_code == 200:
                results["GET_success"] += 1
            results["GET_total"] += 1

        except requests.RequestException as e:
            print(f"Thread-{thread_id} encountered an error: {e}")
            continue

if __name__ == "__main__":
    threads = []
    results = {
        "PUT_success": 0,
        "PUT_total": 0,
        "GET_success": 0,
        "GET_total": 0,
    }

    start_time = time.perf_counter()

    for i in range(NUM_THREADS):
        t = threading.Thread(target=worker, args=(i, results))
        threads.append(t)
        t.start()

    for t in threads:
        t.join()

    end_time = time.perf_counter()

    duration_sec = end_time - start_time
    total_ops = results["PUT_total"] + results["GET_total"]

    print("\nREST API Benchmark Results")
    print("===========================")
    print(f"Total Requests: {total_ops}")
    print(f"Concurrency Level: {NUM_THREADS}")
    print(f"Time taken for tests: {duration_sec:.2f} seconds")
    print(f"Requests per second: {total_ops / duration_sec:.2f} req/sec\n")

    print("Detailed Results:")
    print(f"PUT Requests: {results['PUT_total']} | Successful PUTs: {results['PUT_success']}")
    print(f"GET Requests: {results['GET_total']} | Successful GETs: {results['GET_success']}")
