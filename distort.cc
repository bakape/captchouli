#include "distort.hh"
#include <algorithm>
#include <array>
#include <functional>
#include <random>

using Filter
    = std::function<void(cv::Mat& src, cv::Mat& dst, std::mt19937& rng)>;

static int random_int(std::mt19937& rng, int min, int max)
{
    std::uniform_int_distribution<> dis(min, max);
    return dis(rng);
}

static double random_double(std::mt19937& rng, double min, double max)
{
    std::uniform_real_distribution<> dis(min, max);
    return dis(rng);
}

static void flip(cv::Mat& src, cv::Mat& dst, std::mt19937& rng)
{
    if (random_int(rng, 0, 1)) {
        cv::swap(src, dst);
    } else {
        cv::flip(src, dst, 1);
    }
}

static void gaussian_blur(cv::Mat& src, cv::Mat& dst, std::mt19937& rng)
{
    cv::GaussianBlur(src, dst, cv::Size(), random_double(rng, 0.25, 0.5));
}

static const std::array<Filter, 2> filters = { flip, gaussian_blur };

void cpli_distort_mat(cv::Mat& src, cv::Mat& dst)
{
    std::random_device rd;
    std::mt19937 rng(rd());

    // Always keep the resulting Mat in dst and swap before a new operation
    auto swap = [&]() { cv::swap(src, dst); };

    auto fil = filters;
    std::shuffle(fil.begin(), fil.end(), rng);
    swap();
    for (auto& f : fil) {
        swap();
        f(src, dst, rng);
    }
}
